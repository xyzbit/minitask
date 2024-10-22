package scheduler

import (
	"fmt"
	"runtime"
	"strconv"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

const (
	cpuUsageKey    = "cpu_usage"
	memTotalKey    = "mem_total"
	memUsedKey     = "mem_used"
	memUsageKey    = "mem_usage"
	goGcPauseKey   = "go_gc_pause"
	goGcCountKey   = "go_gc_count"
	goGoroutineKey = "go_goroutine"
)

func GetResourceUsage() (map[string]string, error) {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("获取 CPU 使用率失败: %v", err)
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("获取内存信息失败: %v", err)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]string{
		cpuUsageKey:    strconv.FormatFloat(cpuPercent[0], 'f', 2, 64),
		memTotalKey:    strconv.FormatFloat(float64(memInfo.Total)/(1024*1024*1024), 'f', 2, 64),
		memUsedKey:     strconv.FormatFloat(float64(memInfo.Used)/(1024*1024*1024), 'f', 2, 64),
		memUsageKey:    strconv.FormatFloat(memInfo.UsedPercent, 'f', 2, 64),
		goGcPauseKey:   strconv.FormatFloat(float64(memStats.PauseTotalNs), 'f', 2, 64),
		goGcCountKey:   strconv.FormatFloat(float64(memStats.NumGC), 'f', 2, 64),
		goGoroutineKey: strconv.FormatFloat(float64(runtime.NumGoroutine()), 'f', 2, 64),
	}, nil
}

func ParseResourceUsage(metadata map[string]string) map[string]float64 {
	result := make(map[string]float64)
	for key, value := range metadata {
		floatValue, _ := strconv.ParseFloat(value, 64)
		result[key] = floatValue
	}
	return result
}
