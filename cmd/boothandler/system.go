package main

import (
	"syscall"
	"os"
	"strings"
	"strconv"
	"time"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

const (
	BytesInGB = 1073741824
)

// GetMemoryUsage returns memory usage information in bytes
func systemGetMemoryUsage() (used float64, total float64, err error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, err
	}
	used = float64(v.Used) / BytesInGB
	total = float64(v.Total) / BytesInGB

	return used, total, nil
}

// GetCPUUsage returns CPU usage as a percentage
func systemGetCPUUsage() (float64, error) {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, err
	}
	
	if len(percent) == 0 {
		return 0, nil
	}
	
	return percent[0], nil
}

// GetMemoryUsageGB returns memory usage information in gigabytes
func systemManualGetMemoryUsage() (usedGB float64, totalGB float64, err error) {

	var info syscall.Sysinfo_t
	err = syscall.Sysinfo(&info)
	if err != nil {
		return 0, 0, err
	}
	
	// Convert to bytes first
	totalBytes := info.Totalram * uint64(info.Unit)
	freeBytes := info.Freeram * uint64(info.Unit)
	usedBytes := totalBytes - freeBytes
	
	// Convert to GB
	usedGB = float64(usedBytes) / BytesInGB
	totalGB = float64(totalBytes) / BytesInGB
	
	return usedGB, totalGB, nil
}

// GetCPUUsage returns CPU usage as a percentage
func systemManualGetCPUUsage() (float64, error) {
	// Get initial CPU stats
	idle0, total0, err := getCPUSample()
	if err != nil {
		return 0, err
	}
	
	// Wait for a short period
	time.Sleep(200 * time.Millisecond)
	
	// Get final CPU stats
	idle1, total1, err := getCPUSample()
	if err != nil {
		return 0, err
	}
	
	idleTicks := idle1 - idle0
	totalTicks := total1 - total0
	
	// Calculate CPU usage percentage
	if totalTicks == 0 {
		return 0, nil
	}
	
	return 100 * (1 - float64(idleTicks)/float64(totalTicks)), nil
}

// getCPUSample reads /proc/stat and returns idle and total ticks
func getCPUSample() (idle uint64, total uint64, err error) {
	// For Linux, read from /proc/stat
	contents, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "cpu" {
			// CPU line found
			total = 0
			for i := 1; i < len(fields); i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					continue
				}
				total += val
				if i == 4 { // 4th field is idle time
					idle = val
				}
			}
			break
		}
	}
	
	return idle, total, nil
}
