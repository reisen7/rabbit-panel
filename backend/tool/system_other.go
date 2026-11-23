//go:build !linux && !windows

package tool

import "fmt"

// GetCPUUsage 当前平台不支持
func GetCPUUsage() (float64, error) {
	return 0, fmt.Errorf("system stats not supported on this platform")
}

// GetMemoryUsage 当前平台不支持
func GetMemoryUsage() (MemInfo, error) {
	return MemInfo{}, fmt.Errorf("system stats not supported on this platform")
}

// GetDiskUsage 当前平台不支持
func GetDiskUsage() (DiskInfo, error) {
	return DiskInfo{}, fmt.Errorf("system stats not supported on this platform")
}
