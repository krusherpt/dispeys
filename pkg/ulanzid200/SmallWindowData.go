package ulanzid200

import (
	"math"
	"sync"
	"time"

	hwmonitor "github.com/bjaka-max/dispeys/pkg/hw_monitor"
)

type SmallWindowMode int

const (
	STATS SmallWindowMode = iota
	CLOCK
	BACKGROUND
)

type SmallWindowData struct {
	Mode SmallWindowMode
	CPU  int
	MEM  int
	GPU  int
	Time string
}

var (
	cachedCPU      float64
	cachedCPUTime  time.Time
	cachedMEM      float64
	cachedMEMTime  time.Time
	cachedGPU      float64
	cachedGPUTime  time.Time
	cpuInitialized bool
	mu             sync.Mutex
)

func InitCachedMetrics() {
	mu.Lock()
	defer mu.Unlock()
	if cpuInitialized {
		return
	}
	cpuInitialized = true

	// Warm up CPU cache in background (blocks 1s, but only once)
	go func() {
		cpuPct, _ := hwmonitor.GetCPUUsage()
		mu.Lock()
		cachedCPU = cpuPct
		cachedCPUTime = time.Now()
		mu.Unlock()
	}()

	// MEM and GPU are instant — no blocking
	memPct, _ := hwmonitor.GetMemoryUsage()
	cachedMEM = memPct
	cachedMEMTime = time.Now()

	gpuPct, _ := hwmonitor.GetGPUUsage()
	cachedGPU = gpuPct
	cachedGPUTime = time.Now()
}

func getCPUUsage() float64 {
	mu.Lock()
	defer mu.Unlock()
	// cpu.Percent blocks for 1s, so cache with 1s TTL
	if time.Since(cachedCPUTime) < time.Second {
		return cachedCPU
	}
	cpuPct, err := hwmonitor.GetCPUUsage()
	if err != nil {
		return cachedCPU // stale but non-blocking
	}
	cachedCPU = cpuPct
	cachedCPUTime = time.Now()
	return cachedCPU
}

func getMemoryUsage() float64 {
	mu.Lock()
	defer mu.Unlock()
	// mem.VirtualMemory is instant, but cache to avoid repeated syscall
	if time.Since(cachedMEMTime) < 5*time.Second {
		return cachedMEM
	}
	memPct, err := hwmonitor.GetMemoryUsage()
	if err != nil {
		return cachedMEM
	}
	cachedMEM = memPct
	cachedMEMTime = time.Now()
	return cachedMEM
}

func getGPUUsage() float64 {
	mu.Lock()
	defer mu.Unlock()
	// nvidia-smi is instant, cache for 5s
	if time.Since(cachedGPUTime) < 5*time.Second {
		return cachedGPU
	}
	gpuPct, err := hwmonitor.GetGPUUsage()
	if err != nil {
		return cachedGPU
	}
	cachedGPU = gpuPct
	cachedGPUTime = time.Now()
	return cachedGPU
}

func NewSmallWindowData(data map[string]interface{}) SmallWindowData {
	// Ensure metrics are initialized (warm up happens in background)
	InitCachedMetrics()

	if _, ok := data["time"]; !ok {
		now := time.Now().Format("15:04:05")
		data["time"] = now
	}
	if _, ok := data["mode"]; !ok {
		data["mode"] = CLOCK
	}
	if _, ok := data["cpu"]; !ok {
		data["cpu"] = int(math.Round(getCPUUsage()))
	}
	if _, ok := data["mem"]; !ok {
		data["mem"] = int(math.Round(getMemoryUsage()))
	}
	if _, ok := data["gpu"]; !ok {
		data["gpu"] = int(math.Round(getGPUUsage()))
	}

	return SmallWindowData{
		Mode: data["mode"].(SmallWindowMode),
		CPU:  data["cpu"].(int),
		MEM:  data["mem"].(int),
		GPU:  data["gpu"].(int),
		Time: data["time"].(string),
	}
}

func GetNextMode(mode SmallWindowMode) SmallWindowMode {
	nextMode := (int(mode)+2) % 3
	return SmallWindowMode(nextMode)
}