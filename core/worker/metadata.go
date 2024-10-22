package worker

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

const (
	cpuUsageKey    = "rs_cpu_usage"
	memTotalKey    = "rs_mem_total"
	memUsedKey     = "rs_mem_used"
	memUsageKey    = "rs_mem_usage"
	goGcPauseKey   = "rs_go_gc_pause"
	goGcCountKey   = "rs_go_gc_count"
	goGoroutineKey = "rs_go_goroutine"

	staintPressureCPU = "staint_pressure_cpu"
	staintPressureMem = "staint_pressure_mem"
)

func generateInstanceMetadata() (map[string]string, error) {
	metadata := make(map[string]string)

	workerDesc := generateWorkerDesc()
	for k, v := range workerDesc {
		metadata[k] = v
	}

	ru, err := generateResourceUsage()
	if err != nil {
		return nil, err
	}
	for k, v := range ru {
		metadata[k] = v
	}

	staint, err := generateStaint(ru)
	if err != nil {
		return nil, err
	}
	for k, v := range staint {
		metadata[k] = v
	}

	return metadata, nil
}

func generateResourceUsage() (map[string]string, error) {
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

// 生成污点标签
func generateStaint(ru map[string]string) (map[string]string, error) {
	u := LoadResourceUsage(ru)
	staint := map[string]string{}
	if u[memUsageKey] > 85 {
		staint[staintPressureMem] = "high"
	}
	if u[cpuUsageKey] > 85 {
		staint[staintPressureCPU] = "high"
	}
	return staint, nil
}

// 获取节点描述
func generateWorkerDesc() map[string]string {
	// TODO name, createtime ...
	return nil
}

func LoadResourceUsage(metadata map[string]string) map[string]float64 {
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

func LoadStaint(metadata map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range metadata {
		if !strings.HasPrefix(key, "staint_") {
			continue
		}
		result[key] = value
	}
	return result
}

func LoadWorkerDesc(metadata map[string]string) map[string]string {
	// TODO name, createtime ...
	return nil
}
