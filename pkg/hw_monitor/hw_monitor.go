package hwmonitor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

func GetCPUUsage() (float64, error) {
	percentages, err := cpu.Percent(time.Second, false)
	if err != nil || len(percentages) == 0 {
		return 0, err
	}
	return percentages[0], nil
}

func GetMemoryUsage() (float64, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return v.UsedPercent, nil
}

func GetGPUUsage() (float64, error) {
	out, err := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return 0, fmt.Errorf("nvidia-smi не найден или не установлен")
	}
	str := strings.TrimSpace(string(out))
	value, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, fmt.Errorf("не удалось распарсить вывод nvidia-smi: %v", err)
	}
	return value, nil
}