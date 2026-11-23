//go:build windows

package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

// memoryStatusEx 对应 Win32 MEMORYSTATUSEX
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// GetCPUUsage 获取 CPU 使用率。
// GetSystemTimes 返回自启动以来的累计时间，与 Linux /proc/stat 语义一致；
// kernelTime 已包含 idleTime。
func GetCPUUsage() (float64, error) {
	var idleTime, kernelTime, userTime windows.Filetime
	ret, _, err := kernel32.NewProc("GetSystemTimes").Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret == 0 {
		return 0, err
	}

	idle := uint64(idleTime.Nanoseconds())
	total := uint64(kernelTime.Nanoseconds()) + uint64(userTime.Nanoseconds())
	if total == 0 {
		return 0, nil
	}

	usage := 100.0 * (1.0 - float64(idle)/float64(total))
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
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	ret, _, err := kernel32.NewProc("GlobalMemoryStatusEx").Call(
		uintptr(unsafe.Pointer(&status)),
	)
	if ret == 0 {
		return MemInfo{}, err
	}
	if status.TotalPhys == 0 {
		return MemInfo{}, fmt.Errorf("cannot read memory info")
	}

	used := status.TotalPhys - status.AvailPhys
	usage := 100.0 * float64(used) / float64(status.TotalPhys)
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return MemInfo{Usage: usage, Used: used / 1024, Total: status.TotalPhys / 1024}, nil
}

// GetDiskUsage 获取磁盘使用率（工作目录所在卷）
func GetDiskUsage() (DiskInfo, error) {
	wd, err := os.Getwd()
	if err != nil {
		return DiskInfo{}, err
	}
	root, err := windows.UTF16PtrFromString(filepath.VolumeName(wd) + `\`)
	if err != nil {
		return DiskInfo{}, err
	}

	var freeAvail, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(root, &freeAvail, &total, &totalFree); err != nil {
		return DiskInfo{}, err
	}
	if total == 0 {
		return DiskInfo{}, fmt.Errorf("cannot read disk info")
	}

	used := total - totalFree
	usage := 100.0 * float64(used) / float64(total)
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return DiskInfo{Usage: usage, Used: used / 1024, Total: total / 1024}, nil
}
