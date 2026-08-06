package main

import (
	"fmt"
	"time"
	"os"
	"bufio"
	"strconv"
	"strings"
	"context"
	"sync"

	"github.com/joho/godotenv"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	// "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/disk"
)

type ErrorCategory int

const (
	TYPE_ERROR ErrorCategory = iota
	TYPE_WARNING
)

type AppError struct {
	ErrorType ErrorCategory
	Text string
}

func (e *AppError) Error() string {
	prefix := "ERROR"
	switch e.ErrorType {
	case TYPE_WARNING:
		prefix = "WARNING"
	// place for future possible error_types
	}
	return fmt.Sprintf("%s: %s", prefix, e.Text)
}

type metricsRow struct {
	Time time.Time `ch:"time"`
    Host string `ch:"host"`
    Uptime uint64 `ch:"uptime"`

    CPUUser *float32 `ch:"cpu_user_pct"`
    CPUSystem *float32 `ch:"cpu_system_pct"`
    CPUIdle *float32 `ch:"cpu_idle_pct"`
    CPUSteal *float32 `ch:"cpu_steal_pct"`

    RAMTotal *uint64 `ch:"ram_total_bytes"`
    RAMUsed *uint64 `ch:"ram_used_bytes"`
    RAMFree *uint64 `ch:"ram_free_bytes"`
    RAMAvailable *uint64 `ch:"ram_available_bytes"`

    DiskUtil float32 `ch:"disk_util_pct"`
    DiskUsed uint64 `ch:"fs_used_bytes"`
    DiskFree uint64 `ch:"fs_free_bytes"`
    DiskUsedPercent float32 `ch:"fs_used_pct"`

    NetRxDelta uint64 `ch:"net_rx_bytes_delta"`
    NetTxDelta uint64 `ch:"net_tx_bytes_delta"`
    NetRxPerSec float32 `ch:"net_rx_bytes_per_sec"`
    NetTxPerSec float32 `ch:"net_tx_bytes_per_sec"`
    
    ProcessTotal uint32 `ch:"total_processes"`
    ProcessRunning uint32 `ch:"process_running"`
    ProcessSleeping  uint32 `ch:"process_sleeping"`
    ProcessZombie uint32 `ch:"process_zombie"`
    ProcessStopped uint32 `ch:"process_stopped"`
}

func getHostInfo(row *metricsRow) error {
	info, err := host.Info()
	if err != nil {
		row.Host = "__unknown__"
		row.Uptime = 0
		return &AppError{ErrorType:TYPE_ERROR, Text:fmt.Sprint(err)}
	}	
	row.Host = info.Hostname
	row.Uptime = info.Uptime
	return nil
}

type ticksCPU struct {
	warm bool
	user uint64
	nice uint64
	system uint64
	idle uint64
	iowait uint64
	irq uint64
	softirq uint64
	steal uint64
}

func (t *ticksCPU) getTotal()  uint64 {
	return t.user + t.nice + t.system + t.idle + t.iowait + t.irq + t.softirq + t.steal
}

func fillCPUTicks(t []string) (ticksCPU, error) {
	vals := make([]uint64, 8)
	for i, val := range t {
		v, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return ticksCPU{}, err
		}
		vals[i] = v
	}
	return ticksCPU{
		warm: true,
		user: vals[0],
		nice: vals[1],
		system: vals[2],
		idle: vals[3],
		iowait: vals[4],
		irq: vals[5],
		softirq: vals[6],
		steal: vals[7],
	}, nil
}

func getCPUUsage(prev uint64, curr uint64, prevTotal uint64, currTotal uint64) float32 {
	diffTotal := float32(currTotal) - float32(prevTotal)
	diff := float32(curr) - float32(prev)
	return 100*(diff/diffTotal)
}

var prevTicks ticksCPU

func getCPUStats(row *metricsRow) error {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return err
	}
	defer file.Close()
	
	var info string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		info = scanner.Text()
		break
	}
	
	if err := scanner.Err(); err != nil {
		return err
	}

	currTicks, err := fillCPUTicks(strings.Fields(info)[1:9])

	if !prevTicks.warm || !currTicks.warm {
		prevTicks = currTicks
		return err
	}

	currTotal := currTicks.getTotal()
	prevTotal := prevTicks.getTotal()

	cpuUser := getCPUUsage(prevTicks.user, currTicks.user, prevTotal, currTotal)
	cpuSystem := getCPUUsage(prevTicks.system, currTicks.system, prevTotal, currTotal)
	cpuIdle := getCPUUsage(prevTicks.idle, currTicks.idle, prevTotal, currTotal)
	cpuSteal := getCPUUsage(prevTicks.steal, currTicks.steal, prevTotal, currTotal)

	row.CPUUser = &cpuUser
	row.CPUSystem = &cpuSystem
	row.CPUIdle = &cpuIdle
	row.CPUSteal = &cpuSteal

	prevTicks = currTicks
	return nil
}

func getRAMStats(row *metricsRow) error {
	ramInfo, err := mem.VirtualMemory()
	if err != nil {
		return &AppError{ErrorType: TYPE_ERROR, Text:fmt.Sprint(err)}
	}
	row.RAMTotal = &ramInfo.Total
	row.RAMUsed = &ramInfo.Used
	row.RAMFree = &ramInfo.Free
	row.RAMAvailable = &ramInfo.Available
	return nil
}

func getDiskStats(row *metricsRow) error {
	usageInfo, err := disk.Usage("/")
	if err != nil {
		return &AppError{ErrorType: TYPE_ERROR, Text:fmt.Sprint(err)}
	}
	row.DiskUsed = usageInfo.Used
	row.DiskFree = usageInfo.Free
	row.DiskUsedPercent = float32(usageInfo.UsedPercent)
	return nil
}

func getProcessesStats(row *metricsRow) error {
	entr, err := os.ReadDir("/proc")
	if err != nil {
		return &AppError{ErrorType: TYPE_ERROR, Text: fmt.Sprint(err)}
	}
	var errorCount uint64 = 0

	for _, proc := range entr {
		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}
		row.ProcessTotal++

		procInfo, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			errorCount++
			continue
		}
		
		parenIdx := strings.LastIndex(string(procInfo), ")")
		if parenIdx == -1 {
			errorCount++
			continue
		}

		status := strings.Fields(string(procInfo[parenIdx + 1:]))[0]
		switch status {
		case "R": 
			row.ProcessRunning++
		case "Z":
			row.ProcessZombie++
		case "T", "t":
			row.ProcessStopped++
		case "S", "D":
			row.ProcessSleeping++
		// default:
		// 	processesReturn.Other++
		}
	}
	if errorCount > 0 {
		return &AppError{ErrorType: TYPE_WARNING, Text: fmt.Sprintf("you have %d processes with no status", errorCount)}
	}
	return nil
}

func collectMetrics() (metricsRow, error) {
	// время сбора метрик
	var row metricsRow
	row.Time = time.Now()
	
	// имя хоста, аптайм
	err := getHostInfo(&row)
	if err != nil {
		fmt.Println(err)
		return metricsRow{}, err
	} 
	
	// нагрузка на процессор, вызов занимает 1 секунду для рассчёта
	err = getCPUStats(&row)
	if err != nil {
		fmt.Println(err)
	}
	
	// занятость озу
	err = getRAMStats(&row)
	if err != nil {
		fmt.Println(err)
	}

	// заполненность диска
	err = getDiskStats(&row)
	if err != nil {
		fmt.Println(err)
	}

	// текущие процессы на диске 
	err = getProcessesStats(&row)
	if err != nil {
		fmt.Println(err)
	}

	return row, nil
}

func connect() (driver.Conn, error) {
	var (
        ctx       = context.Background()
        conn, err = clickhouse.Open(&clickhouse.Options{
            Addr: []string{fmt.Sprintf("%s:%s", os.Getenv("CH_HOST"), os.Getenv("CH_PORT"))},
            Auth: clickhouse.Auth{
                Database: os.Getenv("DB_NAME"),
                Username: os.Getenv("DB_USER"),
                Password: os.Getenv("DB_PSWD"),
            },
        })
    )
	
	if err != nil {
        return nil, err
    }

    if err := conn.Ping(ctx); err != nil {
        if exception, ok := err.(*clickhouse.Exception); ok {
            fmt.Printf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
        }
        return nil, err
    }
    return conn, nil
}

func (row *metricsRow) isCPUReady() bool {
	return row.CPUUser == nil || row.CPUIdle == nil || row.CPUSystem == nil || row.CPUSteal == nil
}

func runCollector(metricsCh chan<- metricsRow) {
	collectorTimerSec, _ := strconv.Atoi(os.Getenv("COLLECT_TIMER"))
	ticker := time.NewTicker(time.Duration(collectorTimerSec)*time.Second)
	defer ticker.Stop()
	defer close(metricsCh)

	for range ticker.C{
		row, err := collectMetrics()
		if err != nil {
			fmt.Println("Collecting error", err)
			continue
		}
		if row.isCPUReady() {
			fmt.Println("CPU is not ready")
			continue
		}
		metricsCh <- row
	}
}

func flushBatch(conn driver.Conn, batchBuffer []metricsRow) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	batch, err := conn.PrepareBatch(ctx, "INSERT INTO system_stats.sys_metrics")
	if err != nil {
		panic(err)
	}
	defer batch.Close()

	for _, row := range batchBuffer {
		if err := batch.AppendStruct(&row); err != nil {
			fmt.Println("Could not append row", err)
			continue
		}
	}

	return batch.Send()

}

func runSender(metricsCh <-chan metricsRow, conn driver.Conn) {
	var batchBuffer []metricsRow
	batchSize, _ := strconv.Atoi(os.Getenv("BATCH_SIZE"))
	for row := range metricsCh {
		batchBuffer = append(batchBuffer, row)
		if len(batchBuffer) >= batchSize {
			err := flushBatch(conn, batchBuffer)
			if err != nil {
				fmt.Println("Could not send batch: ", err)
			}
			batchBuffer = batchBuffer[:0]
		}
	}
	
	if err := flushBatch(conn, batchBuffer); err != nil {
		fmt.Println("Could not send final batch: ", err)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
	
	conn, err := connect()
    if err != nil {
        panic(err)
    }
	defer conn.Close() 

	metricsCh := make(chan metricsRow, 180)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		runCollector(metricsCh)
	}()

	go func() {
		defer wg.Done()
		runSender(metricsCh, conn)
	}()

	wg.Wait()
}

