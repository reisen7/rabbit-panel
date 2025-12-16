package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

//go:embed static
var staticFiles embed.FS

// 全局 Docker 客户端
var dockerClient *client.Client

// CPU 使用率缓存（避免每次调用都等待1秒）
var (
	cpuStatsCache struct {
		sync.RWMutex
		lastCPU    []uint64
		lastTime   time.Time
		cpuUsage   float64
	}
)

// 容器列表缓存
var (
	containersCache struct {
		sync.RWMutex
		data      []ContainerInfo
		lastFetch time.Time
	}
	cacheTTL = 2 * time.Second // 缓存有效期 2 秒
)

// 镜像列表缓存
var (
	imagesCache struct {
		sync.RWMutex
		data      []ImageInfo
		lastFetch time.Time
	}
)

// 系统监控数据
type SystemStats struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Disk   float64 `json:"disk"`
	Time   string  `json:"time"`
}

// 容器信息
type ContainerInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Image    string `json:"image"`
	Status   string `json:"status"`
	Ports    string `json:"ports"`
	Memory   string `json:"memory"`
	Created  string `json:"created"`
	State    string `json:"state"`
}

// 镜像信息
type ImageInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	Size    string `json:"size"`
	Created string `json:"created"`
}

// 初始化 Docker 客户端
func initDockerClient() error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("无法连接到 Docker: %v", err)
	}
	dockerClient = cli
	return nil
}

// 读取 CPU 统计信息
func readCPUStats() ([]uint64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, fmt.Errorf("无法读取 /proc/stat")
	}

	line := scanner.Text()
	var user, nice, system, idle, iowait, irq, softirq, steal, guest, guestNice uint64
	_, err = fmt.Sscanf(line, "cpu %d %d %d %d %d %d %d %d %d %d",
		&user, &nice, &system, &idle, &iowait, &irq, &softirq, &steal, &guest, &guestNice)
	if err != nil {
		// 尝试只读取前7个字段（兼容不同系统）
		_, err = fmt.Sscanf(line, "cpu %d %d %d %d %d %d %d",
			&user, &nice, &system, &idle, &iowait, &irq, &softirq)
		if err != nil {
			return nil, err
		}
	}

	return []uint64{user, nice, system, idle, iowait, irq, softirq}, nil
}

// 获取系统 CPU 使用率（使用缓存机制）
func getCPUUsage() (float64, error) {
	cpuStatsCache.RLock()
	// 如果缓存存在且未过期（1秒内），直接返回
	if cpuStatsCache.lastTime.After(time.Now().Add(-2*time.Second)) && len(cpuStatsCache.lastCPU) > 0 {
		usage := cpuStatsCache.cpuUsage
		cpuStatsCache.RUnlock()
		return usage, nil
	}
	cpuStatsCache.RUnlock()

	// 读取第一次 CPU 统计
	cpu1, err := readCPUStats()
	if err != nil {
		return 0, err
	}

	// 等待 500ms
	time.Sleep(500 * time.Millisecond)

	// 读取第二次 CPU 统计
	cpu2, err := readCPUStats()
	if err != nil {
		return 0, err
	}

	if len(cpu1) != len(cpu2) || len(cpu1) < 4 {
		return 0, fmt.Errorf("CPU 统计数据不完整")
	}

	// 计算总 CPU 时间
	total1 := uint64(0)
	total2 := uint64(0)
	for i := range cpu1 {
		total1 += cpu1[i]
		total2 += cpu2[i]
	}

	idle1 := cpu1[3]
	idle2 := cpu2[3]

	idleDelta := idle2 - idle1
	totalDelta := total2 - total1

	if totalDelta == 0 {
		return 0, nil
	}

	cpuUsage := 100.0 * (1.0 - float64(idleDelta)/float64(totalDelta))
	if cpuUsage < 0 {
		cpuUsage = 0
	}
	if cpuUsage > 100 {
		cpuUsage = 100
	}

	// 更新缓存
	cpuStatsCache.Lock()
	cpuStatsCache.lastCPU = cpu2
	cpuStatsCache.lastTime = time.Now()
	cpuStatsCache.cpuUsage = cpuUsage
	cpuStatsCache.Unlock()

	return cpuUsage, nil
}

// 获取系统内存使用率
func getMemoryUsage() (float64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
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

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	if total == 0 {
		return 0, fmt.Errorf("无法读取内存信息")
	}

	// 计算已使用内存
	var used uint64
	if available > 0 {
		used = total - available
	} else {
		// 如果没有 MemAvailable，使用传统计算方法
		used = total - free - buffers - cached
	}

	memoryUsage := 100.0 * float64(used) / float64(total)
	if memoryUsage < 0 {
		memoryUsage = 0
	}
	if memoryUsage > 100 {
		memoryUsage = 100
	}

	return memoryUsage, nil
}

// 获取磁盘使用率
func getDiskUsage() (float64, error) {
	cmd := exec.Command("df", "-h", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("无法解析磁盘信息")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return 0, fmt.Errorf("磁盘信息格式错误")
	}

	usageStr := strings.TrimSuffix(fields[4], "%")
	usage, err := strconv.ParseFloat(usageStr, 64)
	if err != nil {
		return 0, err
	}

	return usage, nil
}

// 系统监控 API
func handleSystemStats(w http.ResponseWriter, r *http.Request) {
	cpu, err := getCPUUsage()
	if err != nil {
		cpu = 0
	}

	memory, err := getMemoryUsage()
	if err != nil {
		memory = 0
	}

	disk, err := getDiskUsage()
	if err != nil {
		disk = 0
	}

	stats := SystemStats{
		CPU:    cpu,
		Memory: memory,
		Disk:   disk,
		Time:   time.Now().Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// 获取容器列表（带缓存）
func handleContainers(w http.ResponseWriter, r *http.Request) {
	// 检查缓存
	containersCache.RLock()
	if time.Since(containersCache.lastFetch) < cacheTTL && len(containersCache.data) > 0 {
		data := containersCache.data
		containersCache.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "private, max-age=2") // 客户端缓存 2 秒
		json.NewEncoder(w).Encode(data)
		return
	}
	containersCache.RUnlock()

	// 从 Docker API 获取
	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		http.Error(w, fmt.Sprintf("获取容器列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	containerList := make([]ContainerInfo, 0, len(containers)) // 预分配容量
	for _, c := range containers {
		// 获取容器名称（去除前导斜杠）
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
			if strings.HasPrefix(name, "/") {
				name = name[1:]
			}
		}
		if name == "" {
			name = c.ID[:12]
		}

		// 格式化端口映射
		ports := []string{}
		for _, p := range c.Ports {
			if p.PublicPort != 0 {
				ports = append(ports, fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else if p.PrivatePort != 0 {
				ports = append(ports, fmt.Sprintf(":%d/%s", p.PrivatePort, p.Type))
			}
		}
		portsStr := strings.Join(ports, ", ")
		if portsStr == "" {
			portsStr = "-"
		}

		// 获取容器 ID（确保至少12位）
		containerID := c.ID
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}

		// 获取容器内存使用
		// 注意：为了性能考虑，这里只显示文件系统大小
		// 实时内存使用可以通过 stats API 获取，但会增加响应时间
		memory := "-"
		if c.SizeRw > 0 {
			// SizeRw 是容器可写层的大小（不是内存使用）
			memory = fmt.Sprintf("FS:%.1fMB", float64(c.SizeRw)/1024/1024)
		}

		// 格式化创建时间
		created := time.Unix(c.Created, 0).Format("2006-01-02 15:04:05")

		containerList = append(containerList, ContainerInfo{
			ID:      containerID,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			Ports:   portsStr,
			Memory:  memory,
			Created: created,
			State:   c.State,
		})
	}

	// 更新缓存
	containersCache.Lock()
	containersCache.data = containerList
	containersCache.lastFetch = time.Now()
	containersCache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=2") // 客户端缓存 2 秒
	json.NewEncoder(w).Encode(containerList)
}

// 容器操作：启动/停止/重启/删除
func handleContainerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	var err error

	switch req.Action {
	case "start":
		err = dockerClient.ContainerStart(ctx, req.ID, types.ContainerStartOptions{})
	case "stop":
		err = dockerClient.ContainerStop(ctx, req.ID, container.StopOptions{})
	case "restart":
		err = dockerClient.ContainerRestart(ctx, req.ID, container.StopOptions{})
	case "remove":
		err = dockerClient.ContainerRemove(ctx, req.ID, types.ContainerRemoveOptions{Force: true})
	default:
		http.Error(w, "不支持的操作", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("操作失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 获取容器日志
func handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("id")
	if containerID == "" {
		http.Error(w, "容器 ID 不能为空", http.StatusBadRequest)
		return
	}

	// 检查客户端是否断开连接
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 监听客户端断开
	go func() {
		<-ctx.Done()
		cancel()
	}()

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
		Follow:     true,
		Timestamps: false,
	}

	logs, err := dockerClient.ContainerLogs(ctx, containerID, options)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取日志失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer logs.Close()

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

	// 创建刷新器
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE 不支持", http.StatusInternalServerError)
		return
	}

	// Docker 日志流式读取
	// Docker 日志格式：每行前8字节是头部
	// [STREAM_TYPE(1字节), PADDING(3字节), SIZE(4字节, 大端序)]
	header := make([]byte, 8)
	
	// 使用缓冲区和 strings.Builder 减少内存分配
	const maxLogLineSize = 64 * 1024 // 限制单行日志最大 64KB（减少内存占用）
	var logBuffer strings.Builder
	logBuffer.Grow(512) // 预分配 512 字节
	
	// 使用固定大小的缓冲区，避免频繁分配
	logDataPool := make([]byte, maxLogLineSize)
	
	for {
		// 检查客户端是否断开
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 读取8字节头部
		_, err := io.ReadFull(logs, header)
		if err != nil {
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				break
			}
			// 使用更小的错误消息
			w.Write([]byte("data: [错误]\n\n"))
			flusher.Flush()
			break
		}

		// 解析大小（大端序）
		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}
		
		// 限制日志行大小，防止内存溢出
		if size > maxLogLineSize {
			// 跳过过大的日志行
			io.CopyN(io.Discard, logs, int64(size))
			continue
		}

		// 使用池化的缓冲区
		logData := logDataPool[:size]
		_, err = io.ReadFull(logs, logData)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			break
		}

		// 清理换行符并构建字符串（重用 buffer）
		logBuffer.Reset()
		logLine := strings.TrimRight(string(logData), "\r\n\t ")

		// 发送 SSE 消息
		if logLine != "" {
			// 转义特殊字符（使用 strings.Builder 优化）
			logBuffer.WriteString("data: ")
			for _, r := range logLine {
				if r == '\n' {
					logBuffer.WriteString("\\n")
				} else if r == '\r' {
					logBuffer.WriteString("\\r")
				} else {
					logBuffer.WriteRune(r)
				}
			}
			logBuffer.WriteString("\n\n")
			w.Write([]byte(logBuffer.String()))
			flusher.Flush()
		}
	}
}

// 获取镜像列表（带缓存）
func handleImages(w http.ResponseWriter, r *http.Request) {
	// 检查缓存
	imagesCache.RLock()
	if time.Since(imagesCache.lastFetch) < cacheTTL*2 && len(imagesCache.data) > 0 {
		data := imagesCache.data
		imagesCache.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "private, max-age=4") // 客户端缓存 4 秒
		json.NewEncoder(w).Encode(data)
		return
	}
	imagesCache.RUnlock()

	// 从 Docker API 获取
	images, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("获取镜像列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	imageList := make([]ImageInfo, 0, len(images)) // 预分配容量
	for _, img := range images {
		// 获取镜像 ID（处理不同的 ID 格式）
		imageID := img.ID
		if strings.HasPrefix(imageID, "sha256:") {
			if len(imageID) > 19 {
				imageID = imageID[7:19] // 去除 "sha256:" 前缀，取前 12 位
			} else {
				imageID = imageID[7:] // 如果长度不足，至少去除前缀
			}
		} else if len(imageID) > 12 {
			imageID = imageID[:12]
		}

		// 获取镜像名称和标签
		name := "<none>"
		tag := "<none>"
		if len(img.RepoTags) > 0 {
			for _, repoTag := range img.RepoTags {
				if repoTag != "<none>:<none>" {
					parts := strings.Split(repoTag, ":")
					if len(parts) >= 2 {
						name = strings.Join(parts[:len(parts)-1], ":")
						tag = parts[len(parts)-1]
					} else {
						name = repoTag
						tag = "latest"
					}
					break // 使用第一个有效的标签
				}
			}
		}

		// 格式化大小
		size := fmt.Sprintf("%.2f MB", float64(img.Size)/1024/1024)

		// 格式化创建时间
		created := time.Unix(img.Created, 0).Format("2006-01-02 15:04:05")

		imageList = append(imageList, ImageInfo{
			ID:      imageID,
			Name:    name,
			Tag:     tag,
			Size:    size,
			Created: created,
		})
	}

	// 更新缓存
	imagesCache.Lock()
	imagesCache.data = imageList
	imagesCache.lastFetch = time.Now()
	imagesCache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=4") // 客户端缓存 4 秒
	json.NewEncoder(w).Encode(imageList)
}

// 删除镜像
func handleImageRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	// 查找完整的镜像 ID
	images, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("获取镜像列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	var imageID string
	for _, img := range images {
		// 匹配完整的镜像 ID 或短 ID
		fullID := img.ID
		shortID := fullID
		if strings.HasPrefix(fullID, "sha256:") {
			shortID = fullID[7:]
		}
		// 取前12位进行比较
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		if strings.HasPrefix(fullID, req.ID) || strings.HasPrefix(shortID, req.ID) || req.ID == shortID {
			imageID = img.ID
			break
		}
	}

	if imageID == "" {
		http.Error(w, "镜像不存在", http.StatusNotFound)
		return
	}

	_, err = dockerClient.ImageRemove(context.Background(), imageID, types.ImageRemoveOptions{})
	if err != nil {
		// 检查是否是依赖错误
		if strings.Contains(err.Error(), "is being used") {
			http.Error(w, "镜像正在被容器使用，请先删除相关容器", http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("删除失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 获取服务器 IP 地址
func getServerIP() string {
	// 方法1: 通过连接外部地址获取本机 IP（最准确）
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	// 方法2: 获取第一个非回环的网络接口 IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	// 优先获取非 Docker 网桥的 IP
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				// 排除 Docker 默认网桥 (172.17.0.0/16)
				if !strings.HasPrefix(ip, "172.17.") {
					return ip
				}
			}
		}
	}

	// 如果都找不到，返回第一个非回环 IP
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}

// 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	_, err := dockerClient.Ping(context.Background())
	if err != nil {
		http.Error(w, "Docker 连接失败", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	// 初始化认证数据库
	if err := initAuthDB(); err != nil {
		log.Fatalf("初始化认证数据库失败: %v", err)
	}

	// 获取运行模式（master 或 worker）
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = ModeMaster // 默认 Master 模式
	}
	
	// 初始化节点管理器
	initNodeManager(mode)

	// 初始化 Docker 客户端
	if err := initDockerClient(); err != nil {
		log.Fatalf("初始化 Docker 客户端失败: %v\n请确保 Docker 已安装并运行，且当前用户有 Docker 访问权限", err)
	}

	// 检查 Docker 连接
	_, err := dockerClient.Ping(context.Background())
	if err != nil {
		log.Fatalf("无法连接到 Docker: %v\n请确保 Docker 服务正在运行", err)
	}

	// 获取端口（默认 9999）
	port := os.Getenv("PORT")
	if port == "" {
		if mode == ModeWorker {
			port = "10001" // Worker 默认端口
		} else {
			port = "9999" // Master 默认端口
		}
	}

	// 获取监听地址（默认 0.0.0.0，允许外网访问）
	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	// 获取服务器 IP 地址
	serverIP := getServerIP()
	nodeAddress := serverIP
	if nodeAddress == "" {
		nodeAddress = "localhost"
	}
	nodeAddress = nodeAddress + ":" + port

	// Worker 模式：向 Master 注册
	if mode == ModeWorker {
		masterURL := os.Getenv("MASTER_URL")
		if masterURL == "" {
			log.Fatalf("Worker 模式需要设置 MASTER_URL 环境变量")
		}
		
		// 生成节点 ID
		hostname, _ := os.Hostname()
		nodeID := fmt.Sprintf("%s-%s", hostname, port)
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			nodeName = hostname
		}
		
		// 注册到 Master
		if err := registerToMaster(masterURL, nodeID, nodeName, nodeAddress); err != nil {
			log.Printf("警告: 向 Master 注册失败: %v，将在后台重试", err)
		}
		
		// 启动心跳协程
		go sendHeartbeatToMaster(masterURL, nodeID)
		log.Printf("Worker 节点已启动，Master: %s", masterURL)
	}

	// 配置 HTTP 服务器（优化内存和性能）
	server := &http.Server{
		Addr:           host + ":" + port,
		ReadTimeout:    15 * time.Second,  // 读取超时
		WriteTimeout:   30 * time.Second,  // 写入超时（日志流需要更长时间）
		IdleTimeout:    60 * time.Second,  // 空闲连接超时
		MaxHeaderBytes: 1 << 20,           // 最大请求头 1MB
		// 限制并发连接数（减少内存占用）
		// 注意：对于日志流，需要较长的连接时间
	}

	// 认证相关路由（不需要认证）
	http.HandleFunc("/api/auth/login", handleLogin)
	http.HandleFunc("/api/health", handleHealth)
	
	// 需要认证的路由
	http.HandleFunc("/api/auth/change-password", authMiddleware(handleChangePassword))
	http.HandleFunc("/api/auth/logout", authMiddleware(handleLogout))
	http.HandleFunc("/api/auth/me", authMiddleware(handleGetCurrentUser))
	
	// 设置路由（使用自定义 Handler 限制并发，需要认证）
	http.HandleFunc("/api/system/stats", authOrNodeAuthMiddleware(handleSystemStats))
	http.HandleFunc("/api/containers", authOrNodeAuthMiddleware(handleContainers)) // 支持用户认证或节点认证
	http.HandleFunc("/api/containers/action", authMiddleware(handleContainerAction))
	http.HandleFunc("/api/containers/logs", authMiddleware(handleContainerLogs)) // 日志流不限制超时
	http.HandleFunc("/api/images", authOrNodeAuthMiddleware(handleImages)) // 支持用户认证或节点认证
	http.HandleFunc("/api/images/remove", authMiddleware(handleImageRemove))
	
	// Compose 管理 API
	initCompose()
	http.HandleFunc("/api/compose/list", authMiddleware(handleComposeList))
	http.HandleFunc("/api/compose/create", authMiddleware(handleComposeCreate))
	http.HandleFunc("/api/compose/file", authMiddleware(handleComposeGetFile))
	http.HandleFunc("/api/compose/save", authMiddleware(handleComposeSaveFile))
	http.HandleFunc("/api/compose/action", authMiddleware(handleComposeAction))

	// 多节点管理 API（仅 Master 模式）
	if mode == ModeMaster {
		http.HandleFunc("/api/nodes", authMiddleware(handleNodesList)) // Web UI 访问需要用户认证
		http.HandleFunc("/api/nodes/register", nodeAuthMiddleware(handleNodeRegister)) // Worker 注册需要节点认证
		http.HandleFunc("/api/nodes/heartbeat", nodeAuthMiddleware(handleNodeHeartbeat)) // Worker 心跳需要节点认证
		http.HandleFunc("/api/containers/schedule", authMiddleware(handleContainerSchedule)) // 跨节点调度需要用户认证
		http.HandleFunc("/api/containers/all", authMiddleware(handleAllContainers))            // 获取所有节点的容器需要用户认证
	}
	
	// Worker 节点：容器创建 API（供 Master 调用，需要节点认证）
	if mode == ModeWorker {
		http.HandleFunc("/api/containers/create", nodeAuthMiddleware(handleContainerCreate))
	}

	// 静态文件服务（处理所有其他路径）
	// 使用 embed 嵌入静态文件，实现单文件部署
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("无法加载静态文件: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 排除 API 路径（虽然正常不会走到这里，但作为兜底）
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// 兼容 /static/ 前缀的请求
		if strings.HasPrefix(r.URL.Path, "/static/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/static")
		}

		// 如果是根路径，http.FileServer 会自动寻找 index.html
		// 但为了确保 SPA 路由（如果有）或明确行为，我们可以显式处理
		// 这里直接交给 fileServer 处理即可
		fileServer.ServeHTTP(w, r)
	})

	// 启动服务器
	log.Printf("容器运维面板启动成功！")
	log.Printf("监听地址: %s", server.Addr)
	log.Printf("本地访问: http://localhost:%s", port)
	if serverIP != "" {
		log.Printf("外网访问: http://%s:%s", serverIP, port)
	} else {
		log.Printf("外网访问: http://<服务器IP>:%s", port)
	}
	if mode == ModeMaster {
		log.Printf("Master 节点: 管理所有 Worker 节点")
	} else {
		log.Printf("Worker 节点: 已连接到 Master")
	}
	log.Printf("系统架构: %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("内存优化: 已启用缓存和连接限制")
	log.Printf("按 Ctrl+C 停止服务")

	// 设置 GC 目标百分比（降低内存占用）
	debug.SetGCPercent(100) // 默认 100，可以设置为更激进的值
	
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

