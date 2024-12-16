package model

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

const (
	CpuUsageKey    = "rs_cpu_usage"
	MemTotalKey    = "rs_mem_total"
	MemUsedKey     = "rs_mem_used"
	MemUsageKey    = "rs_mem_usage"
	GoGcPauseKey   = "rs_go_gc_pause"
	GoGcCountKey   = "rs_go_gc_count"
	GoGoroutineKey = "rs_go_goroutine"

	staintPressureCPU = "staint_pressure_cpu"
	staintPressureMem = "staint_pressure_mem"
	staintDisable     = "staint_disable" // use for mark temporary offline
)

func GenerateResourceUsage() (map[string]string, error) {
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
		CpuUsageKey:    strconv.FormatFloat(cpuPercent[0], 'f', 2, 64),
		MemTotalKey:    strconv.FormatFloat(float64(memInfo.Total)/(1024*1024*1024), 'f', 2, 64),
		MemUsedKey:     strconv.FormatFloat(float64(memInfo.Used)/(1024*1024*1024), 'f', 2, 64),
		MemUsageKey:    strconv.FormatFloat(memInfo.UsedPercent, 'f', 2, 64),
		GoGcPauseKey:   strconv.FormatFloat(float64(memStats.PauseTotalNs), 'f', 2, 64),
		GoGcCountKey:   strconv.FormatFloat(float64(memStats.NumGC), 'f', 2, 64),
		GoGoroutineKey: strconv.FormatFloat(float64(runtime.NumGoroutine()), 'f', 2, 64),
	}, nil
}

// 生成污点标签
func GenerateStaint(ru map[string]string, disable bool) (map[string]string, error) {
	u := ParseResourceUsage(ru)
	staint := map[string]string{}
	if u[MemUsageKey] > 85 {
		staint[staintPressureMem] = "high"
	}
	if u[MemUsageKey] > 85 {
		staint[staintPressureCPU] = "high"
	}
	if disable {
		staint[staintDisable] = "true"
	}
	return staint, nil
}

func ParseResourceUsage(metadata map[string]string) map[string]float64 {
	result := make(map[string]float64)
	for key, value := range metadata {
		if !strings.HasPrefix(key, "rs_") {
			continue
		}
		floatValue, _ := strconv.ParseFloat(value, 64)
		result[key] = floatValue
	}
	return result
}

func ParseStaint(metadata map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range metadata {
		if !strings.HasPrefix(key, "staint_") {
			continue
		}
		result[key] = value
	}
	return result
}
