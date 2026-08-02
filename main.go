package main

import (
	"fmt"
	"time"
	"os"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
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

type hostInfo struct {
	Name string
	Uptime uint64
}

func getHostInfo() (hostInfo, error) {
	info, err := host.Info()
	if err != nil {
		return hostInfo{Name: "__unknown__", Uptime: 0}, &AppError{ErrorType:TYPE_ERROR, Text:fmt.Sprint(err)}
	}	
	return hostInfo{Name: info.Hostname, Uptime: info.Uptime}, nil
}

type statCPU struct {
	Percent float32
}

func getCPUStats() (statCPU, error) {
	cpuPercentBig, err := cpu.Percent(time.Second, false)
	if err != nil || len(cpuPercentBig) == 0 {
		return statCPU{}, &AppError{ErrorType: TYPE_ERROR, Text:"cannot get cpu stats"}
	}
	return statCPU{Percent: float32(cpuPercentBig[0])}, nil
}

type statRAM struct {
	Total uint64
	Used uint64
	Free uint64
	Available uint64
}

func getRAMStats() (statRAM, error) {
	ramInfo, err := mem.VirtualMemory()
	if err != nil {
		return statRAM{}, &AppError{ErrorType: TYPE_ERROR, Text:fmt.Sprint(err)}
	}
	return statRAM{
		Total: ramInfo.Total,
		Used: ramInfo.Used,
		Free: ramInfo.Free,
		Available: ramInfo.Available,
	}, nil
}

type statDisk struct {
	Total uint64
	Used uint64
	Free uint64
	UsedPercent float64
}

func getDiskStats() (statDisk, error) {
	usageInfo, err := disk.Usage("/")
	if err != nil {
		return statDisk{}, &AppError{ErrorType: TYPE_ERROR, Text:fmt.Sprint(err)}
	}
	return statDisk{
		Total: usageInfo.Total,
		Used: usageInfo.Used,
		Free: usageInfo.Free,
		UsedPercent: usageInfo.UsedPercent,
	}, nil
}

type statProcesses struct {
	Total uint64
	Running uint64
	Stopped uint64
	Sleeping uint64
	Zombie uint64
	Other uint64
}

func getProcessesStats() (statProcesses, error) {
	entr, err := os.ReadDir("/proc")
	if err != nil {
		return statProcesses{}, &AppError{ErrorType: TYPE_ERROR, Text: fmt.Sprint(err)}
	}
	var processesReturn statProcesses
	var errorCount uint64 = 0

	for _, proc := range entr {
		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}
		processesReturn.Total++

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
			processesReturn.Running++
		case "Z":
			processesReturn.Zombie++
		case "T", "t":
			processesReturn.Stopped++
		case "S", "D":
			processesReturn.Sleeping++
		default:
			processesReturn.Other++
		}
	}
	if errorCount > 0 {
		return processesReturn, &AppError{ErrorType: TYPE_WARNING, Text: fmt.Sprintf("you have %d processes with no status", errorCount)}
	}
	return processesReturn, nil
}

func main() {

	// время сбора метрик
	currentTime := time.Now()
	fmt.Println(currentTime)

	// имя хоста, аптайм
	hostStat, err := getHostInfo()
	if err != nil {
		fmt.Println(err)
	} 
	fmt.Println(hostStat)

	// нагрузка на процессор, вызов занимает 1 секунду для рассчёта
	cpuStat, err := getCPUStats()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(cpuStat)
	
	// занятость озу
	ramStat, err := getRAMStats()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(ramStat)

	// заполненность диска
	diskStat, err := getDiskStats()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(diskStat)

	// текущие процессы на диске 
	processesStat, err := getProcessesStats()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(processesStat)
	
}

