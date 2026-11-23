package tool

import (
	"runtime"
	"testing"
)

func TestSystemStats(t *testing.T) {
	cpu, cpuErr := GetCPUUsage()
	mem, memErr := GetMemoryUsage()
	disk, diskErr := GetDiskUsage()

	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		if cpuErr == nil || memErr == nil || diskErr == nil {
			t.Fatal("expected errors on unsupported platform")
		}
		return
	}

	if cpuErr != nil {
		t.Fatalf("GetCPUUsage: %v", cpuErr)
	}
	if memErr != nil {
		t.Fatalf("GetMemoryUsage: %v", memErr)
	}
	if diskErr != nil {
		t.Fatalf("GetDiskUsage: %v", diskErr)
	}

	if cpu < 0 || cpu > 100 {
		t.Errorf("cpu usage out of range: %v", cpu)
	}
	if mem.Usage <= 0 || mem.Usage > 100 || mem.Total == 0 {
		t.Errorf("bad memory info: %+v", mem)
	}
	if disk.Usage <= 0 || disk.Usage > 100 || disk.Total == 0 {
		t.Errorf("bad disk info: %+v", disk)
	}
	t.Logf("cpu=%.1f%% mem=%.1f%% (%d/%dKB) disk=%.1f%% (%d/%dKB)",
		cpu, mem.Usage, mem.Used, mem.Total, disk.Usage, disk.Used, disk.Total)
}
