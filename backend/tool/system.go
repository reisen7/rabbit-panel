package tool

// MemInfo 系统内存信息（Used/Total 单位：KB）
type MemInfo struct{ Usage float64; Used uint64; Total uint64 }

// DiskInfo 磁盘使用信息（Used/Total 单位：KB）
type DiskInfo struct{ Usage float64; Used uint64; Total uint64 }
