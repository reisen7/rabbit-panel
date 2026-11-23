//go:build linux

package tool

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// GetCPUUsage 获取 CPU 使用率
func GetCPUUsage() (float64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, fmt.Errorf("cannot read /proc/stat")
	}

	line := scanner.Text()
	var user, nice, system, idle, iowait, irq, softirq uint64
	n, _ := fmt.Sscanf(line, "cpu %d %d %d %d %d %d %d", &user, &nice, &system, &idle, &iowait, &irq, &softirq)
	if n < 4 {
		return 0, fmt.Errorf("invalid /proc/stat format")
	}

	total := user + nice + system + idle + iowait + irq + softirq
	idleTotal := idle + iowait

	if total == 0 {
		return 0, nil
	}

	usage := 100.0 * (1.0 - float64(idleTotal)/float64(total))
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}

	return usage, nil
}

// GetMemoryUsage 获取内存使用率
func GetMemoryUsage() (MemInfo, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemInfo{}, err
	}
	defer file.Close()

	var total, free, available, buffers, cached uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal: %d kB", &total)
		} else if strings.HasPrefix(line, "MemFree:") {
			fmt.Sscanf(line, "MemFree: %d kB", &free)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable: %d kB", &available)
		} else if strings.HasPrefix(line, "Buffers:") {
			fmt.Sscanf(line, "Buffers: %d kB", &buffers)
		} else if strings.HasPrefix(line, "Cached:") {
			fmt.Sscanf(line, "Cached: %d kB", &cached)
		}
	}

	if total == 0 {
		return MemInfo{}, fmt.Errorf("cannot read memory info")
	}

	used := total - available
	if available == 0 {
		used = total - free - buffers - cached
	}

	usage := 100.0 * float64(used) / float64(total)
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}

	return MemInfo{Usage: usage, Used: used, Total: total}, nil
}

// GetDiskUsage 获取磁盘使用率
func GetDiskUsage() (DiskInfo, error) {
	cmd := exec.Command("df", "-k", "/")
	output, err := cmd.Output()
	if err != nil {
		return DiskInfo{}, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return DiskInfo{}, fmt.Errorf("cannot parse disk info")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return DiskInfo{}, fmt.Errorf("disk info format error")
	}

	total, _ := strconv.ParseUint(fields[1], 10, 64)
	used, _ := strconv.ParseUint(fields[2], 10, 64)
	usageStr := strings.TrimSuffix(fields[4], "%")
	usage, _ := strconv.ParseFloat(usageStr, 64)

	return DiskInfo{Usage: usage, Used: used, Total: total}, nil
}
