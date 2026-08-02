CREATE TABLE default.sys_metrics 
(
    `time` DateTime CODEC(DoubleDelta), 
    `host` LowCardinality(String), 
    `uptime` UInt64 CODEC(Delta(8), LZ4), 

    `cpu_user_pct` Float32 CODEC(Gorilla(4)), 
    `cpu_system_pct` Nullable(Float32) CODEC(Gorilla(4)), 
    `cpu_idle_pct` Nullable(Float32) CODEC(Gorilla(4)), 
    `cpu_steal_pct` Nullable(Float32) CODEC(Gorilla(4)), 

    `ram_total_bytes` UInt64, 
    `ram_used_bytes` UInt64 CODEC(Delta(8), LZ4), 
    `ram_free_bytes` UInt64 CODEC(Delta(8), LZ4), 
    `ram_available_bytes` UInt64 CODEC(Delta(8), LZ4), 

    `disk_util_pct` Nullable(Float32) CODEC(Gorilla(4)), 
    `fs_used_bytes` UInt64 CODEC(Delta(8), ZSTD(1)), 
    `fs_free_bytes` UInt64 CODEC(Delta(8), ZSTD(1)), 
    `fs_used_pct` Float32 CODEC(Gorilla(4)), 

    `net_rx_bytes_delta` UInt64 DEFAULT 0 CODEC(Delta(8), ZSTD(1)), 
    `net_tx_bytes_delta` UInt64 DEFAULT 0 CODEC(Delta(8), ZSTD(1)), 
    `net_rx_bytes_per_sec` Float32 DEFAULT 0 CODEC(Gorilla(4), ZSTD(1)), 
    `net_tx_bytes_per_sec` Float32 DEFAULT 0 CODEC(Gorilla(4), ZSTD(1)), 
    
    `total_processes` UInt32 CODEC(T64, ZSTD(1)), 
    `process_running` UInt32 CODEC(T64, ZSTD(1)), 
    `process_sleeping` UInt32 CODEC(T64, ZSTD(1)), 
    `process_zombie` UInt32 CODEC(T64, ZSTD(1)), 
    `process_stopped` UInt32 CODEC(T64, ZSTD(1))
)
ENGINE = MergeTree 
PARTITION BY toYYYYMM(time) 
ORDER BY (host, time) 
TTL toDate(time) + toIntervalDay(60) 
SETTINGS index_granularity = 8192