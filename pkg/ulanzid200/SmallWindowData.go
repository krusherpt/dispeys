package ulanzid200

import (
	"math"
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

func NewSmallWindowData(data map[string]interface{}) SmallWindowData {
	if _, ok := data["time"]; !ok {
		now := time.Now().Format("15:04:05")
		data["time"] = now
	}
	if _, ok := data["mode"]; !ok {
		data["mode"] = CLOCK
	}
	if _, ok := data["cpu"]; !ok {
		cpuUsage, err := hwmonitor.GetCPUUsage()
		if err != nil {
			data["cpu"] = 0
		} else {
			data["cpu"] = int(math.Round(cpuUsage))
		}
	}
	if _, ok := data["mem"]; !ok {
		memoryUsage, err := hwmonitor.GetMemoryUsage()
		if err != nil {
			data["mem"] = 0
		} else {
			data["mem"] = int(math.Round(memoryUsage))
		}
	}
	if _, ok := data["gpu"]; !ok {
		gpuUsage, err := hwmonitor.GetGPUUsage()
		if err != nil {
			data["gpu"] = 0
		} else {
			data["gpu"] = int(math.Round(gpuUsage))
		}
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