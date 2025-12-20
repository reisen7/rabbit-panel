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
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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
	// 使用空版本字符串，让客户端自动协商 API 版本
	// 这样可以同时兼容旧版和新版 Docker
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithVersion(""), // 不指定版本，自动协商
	)
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

// 创建并运行容器 (docker run)
func handleContainerRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Image   string `json:"image"`
		Name    string `json:"name"`
		Restart string `json:"restart"`
		Network string `json:"network"`
		Ports   []struct {
			Host      string `json:"host"`
			Container string `json:"container"`
		} `json:"ports"`
		Envs []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"envs"`
		Volumes []struct {
			Host      string `json:"host"`
			Container string `json:"container"`
		} `json:"volumes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	if req.Image == "" {
		http.Error(w, "镜像名称不能为空", http.StatusBadRequest)
		return
	}

	log.Printf("[Container] Creating container, image: %s, name: %s", req.Image, req.Name)

	ctx := context.Background()

	// 尝试拉取镜像（如果本地没有）
	_, _, err := dockerClient.ImageInspectWithRaw(ctx, req.Image)
	if err != nil {
		// 镜像不存在，尝试拉取
		log.Printf("[Container] Image %s not found, pulling...", req.Image)
		reader, err := dockerClient.ImagePull(ctx, req.Image, types.ImagePullOptions{})
		if err != nil {
			log.Printf("[Container] Failed to pull image: %v", err)
			http.Error(w, fmt.Sprintf("拉取镜像失败: %v", err), http.StatusInternalServerError)
			return
		}
		defer reader.Close()
		// 等待拉取完成
		io.Copy(io.Discard, reader)
		log.Printf("[Container] Image %s pulled successfully", req.Image)
	}

	// 构建容器配置
	config := &container.Config{
		Image: req.Image,
	}

	// 环境变量
	for _, env := range req.Envs {
		if env.Key != "" {
			config.Env = append(config.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
		}
	}

	// 主机配置
	hostConfig := &container.HostConfig{}

	// 端口映射
	if len(req.Ports) > 0 {
		portBindings := make(map[nat.Port][]nat.PortBinding)
		exposedPorts := make(map[nat.Port]struct{})
		for _, p := range req.Ports {
			if p.Host != "" && p.Container != "" {
				containerPort := nat.Port(p.Container + "/tcp")
				exposedPorts[containerPort] = struct{}{}
				portBindings[containerPort] = []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: p.Host},
				}
			}
		}
		config.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}

	// 数据卷
	for _, v := range req.Volumes {
		if v.Host != "" && v.Container != "" {
			hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", v.Host, v.Container))
		}
	}

	// 重启策略
	if req.Restart != "" {
		hostConfig.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(req.Restart)}
	}

	// 网络模式
	if req.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(req.Network)
	}

	// 创建容器
	resp, err := dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, req.Name)
	if err != nil {
		log.Printf("[Container] Failed to create, image: %s, name: %s, error: %v", req.Image, req.Name, err)
		http.Error(w, fmt.Sprintf("创建容器失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 启动容器
	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Printf("[Container] Failed to start, id: %s, error: %v", resp.ID, err)
		// 启动失败，删除已创建的容器
		dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		http.Error(w, fmt.Sprintf("启动容器失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[Container] Created successfully, id: %s, name: %s, image: %s", resp.ID[:12], req.Name, req.Image)

	// 清除容器列表缓存
	containersCache.Lock()
	containersCache.lastFetch = time.Time{}
	containersCache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "id": resp.ID})
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

	log.Printf("[Container] Action: %s, id: %s", req.Action, req.ID)

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
		log.Printf("[Container] Action failed, action: %s, id: %s, error: %v", req.Action, req.ID, err)
		http.Error(w, fmt.Sprintf("操作失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[Container] Action success, action: %s, id: %s", req.Action, req.ID)

	// 清除容器列表缓存，确保下次请求获取最新数据
	containersCache.Lock()
	containersCache.lastFetch = time.Time{}
	containersCache.Unlock()

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

// 获取镜像列表（带缓存，支持 ?refresh=true 强制刷新）
func handleImages(w http.ResponseWriter, r *http.Request) {
	// 检查是否强制刷新
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	// 检查缓存（如果不是强制刷新）
	if !forceRefresh {
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
	}

	// 从 Docker API 获取
	images, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("获取镜像列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	imageList := make([]ImageInfo, 0, len(images)*2) // 预分配容量（一个镜像可能有多个标签）
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

		// 格式化大小
		size := fmt.Sprintf("%.2f MB", float64(img.Size)/1024/1024)

		// 格式化创建时间
		created := time.Unix(img.Created, 0).Format("2006-01-02 15:04:05")

		// 处理所有标签，每个标签生成一条记录
		if len(img.RepoTags) > 0 {
			for _, repoTag := range img.RepoTags {
				if repoTag == "<none>:<none>" {
					continue
				}
				name := "<none>"
				tag := "<none>"
				parts := strings.Split(repoTag, ":")
				if len(parts) >= 2 {
					name = strings.Join(parts[:len(parts)-1], ":")
					tag = parts[len(parts)-1]
				} else {
					name = repoTag
					tag = "latest"
				}
				imageList = append(imageList, ImageInfo{
					ID:      imageID,
					Name:    name,
					Tag:     tag,
					Size:    size,
					Created: created,
				})
			}
		}

		// 如果没有有效标签，添加一条 <none> 记录
		if len(img.RepoTags) == 0 || (len(img.RepoTags) == 1 && img.RepoTags[0] == "<none>:<none>") {
			imageList = append(imageList, ImageInfo{
				ID:      imageID,
				Name:    "<none>",
				Tag:     "<none>",
				Size:    size,
				Created: created,
			})
		}
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

// 构建镜像 (从 Dockerfile)
func handleImageBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ImageName  string `json:"image_name"`  // 镜像名称
		Tag        string `json:"tag"`         // 标签
		Dockerfile string `json:"dockerfile"`  // Dockerfile 内容
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	if req.ImageName == "" {
		http.Error(w, "镜像名称不能为空", http.StatusBadRequest)
		return
	}

	if req.Dockerfile == "" {
		http.Error(w, "Dockerfile 内容不能为空", http.StatusBadRequest)
		return
	}

	if req.Tag == "" {
		req.Tag = "latest"
	}

	// 构建完整的镜像标签
	imageTag := req.ImageName + ":" + req.Tag

	// 创建临时目录作为构建上下文
	tempDir, err := os.MkdirTemp("", "docker-build-")
	if err != nil {
		http.Error(w, fmt.Sprintf("创建临时目录失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// 写入 Dockerfile
	dockerfilePath := tempDir + "/Dockerfile"
	if err := os.WriteFile(dockerfilePath, []byte(req.Dockerfile), 0644); err != nil {
		http.Error(w, fmt.Sprintf("写入 Dockerfile 失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE 不支持", http.StatusInternalServerError)
		return
	}

	// 发送开始消息
	fmt.Fprintf(w, "data: {\"type\":\"start\",\"message\":\"开始构建镜像 %s\"}\n\n", imageTag)
	flusher.Flush()

	// 使用 docker build 命令构建（更简单可靠）
	cmd := exec.Command("docker", "build", "-t", imageTag, tempDir)
	
	// 获取命令输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"获取输出失败: %v\"}\n\n", err)
		flusher.Flush()
		return
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"获取错误输出失败: %v\"}\n\n", err)
		flusher.Flush()
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"启动构建失败: %v\"}\n\n", err)
		flusher.Flush()
		return
	}

	// 读取并发送输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// 转义 JSON 特殊字符
			line = strings.ReplaceAll(line, "\\", "\\\\")
			line = strings.ReplaceAll(line, "\"", "\\\"")
			line = strings.ReplaceAll(line, "\n", "\\n")
			fmt.Fprintf(w, "data: {\"type\":\"log\",\"message\":\"%s\"}\n\n", line)
			flusher.Flush()
		}
	}()

	// 读取错误输出
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.ReplaceAll(line, "\\", "\\\\")
			line = strings.ReplaceAll(line, "\"", "\\\"")
			line = strings.ReplaceAll(line, "\n", "\\n")
			fmt.Fprintf(w, "data: {\"type\":\"log\",\"message\":\"%s\"}\n\n", line)
			flusher.Flush()
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"构建失败: %v\"}\n\n", err)
		flusher.Flush()
		return
	}

	// 清除镜像缓存
	imagesCache.Lock()
	imagesCache.lastFetch = time.Time{}
	imagesCache.Unlock()

	fmt.Fprintf(w, "data: {\"type\":\"success\",\"message\":\"镜像 %s 构建成功！\"}\n\n", imageTag)
	flusher.Flush()
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

	log.Printf("[Image] Remove request, id: %s", req.ID)

	// 直接用传入的 ID 删除（Docker API 支持短 ID）
	deleted, err := dockerClient.ImageRemove(context.Background(), req.ID, types.ImageRemoveOptions{})
	if err != nil {
		log.Printf("[Image] Remove failed, id: %s, error: %v", req.ID, err)
		errMsg := err.Error()
		// 友好的错误提示
		if strings.Contains(errMsg, "is being used") || strings.Contains(errMsg, "using") {
			http.Error(w, "删除失败: 镜像正在被容器使用，请先停止并删除相关容器", http.StatusBadRequest)
			return
		}
		if strings.Contains(errMsg, "has dependent child") || strings.Contains(errMsg, "image has dependent") {
			http.Error(w, "删除失败: 镜像有子镜像依赖，请先删除依赖的镜像", http.StatusBadRequest)
			return
		}
		if strings.Contains(errMsg, "image is referenced") {
			http.Error(w, "删除失败: 镜像被其他镜像引用", http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("删除失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[Image] Remove success, id: %s, result: %+v", req.ID, deleted)

	// 清除镜像缓存
	imagesCache.Lock()
	imagesCache.lastFetch = time.Time{}
	imagesCache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// ========== 网络管理 API ==========

// 网络信息
type NetworkInfo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Scope      string   `json:"scope"`
	IPAM       string   `json:"ipam"`
	Internal   bool     `json:"internal"`
	Containers int      `json:"containers"`
	Created    string   `json:"created"`
}

// 获取网络列表
func handleNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := dockerClient.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("获取网络列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	networkList := make([]NetworkInfo, 0, len(networks))
	for _, n := range networks {
		// 获取网络 ID
		networkID := n.ID
		if len(networkID) > 12 {
			networkID = networkID[:12]
		}

		// 获取 IPAM 配置
		ipam := "-"
		if len(n.IPAM.Config) > 0 {
			ipam = n.IPAM.Config[0].Subnet
		}

		// 格式化创建时间
		created := n.Created.Format("2006-01-02 15:04:05")

		networkList = append(networkList, NetworkInfo{
			ID:         networkID,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			IPAM:       ipam,
			Internal:   n.Internal,
			Containers: len(n.Containers),
			Created:    created,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(networkList)
}

// 创建网络
func handleNetworkCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name     string `json:"name"`
		Driver   string `json:"driver"`
		Subnet   string `json:"subnet"`
		Gateway  string `json:"gateway"`
		Internal bool   `json:"internal"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "网络名称不能为空", http.StatusBadRequest)
		return
	}

	if req.Driver == "" {
		req.Driver = "bridge"
	}

	// 构建 IPAM 配置
	ipamConfig := []network.IPAMConfig{}
	if req.Subnet != "" {
		config := network.IPAMConfig{
			Subnet: req.Subnet,
		}
		if req.Gateway != "" {
			config.Gateway = req.Gateway
		}
		ipamConfig = append(ipamConfig, config)
	}

	options := types.NetworkCreate{
		Driver:   req.Driver,
		Internal: req.Internal,
	}

	if len(ipamConfig) > 0 {
		options.IPAM = &network.IPAM{
			Config: ipamConfig,
		}
	}

	log.Printf("[Network] Creating network, name: %s, driver: %s", req.Name, req.Driver)

	resp, err := dockerClient.NetworkCreate(context.Background(), req.Name, options)
	if err != nil {
		log.Printf("[Network] Create failed, name: %s, error: %v", req.Name, err)
		http.Error(w, fmt.Sprintf("创建网络失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[Network] Created successfully, name: %s, id: %s", req.Name, resp.ID[:12])

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "id": resp.ID})
}

// 删除网络
func handleNetworkRemove(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("[Network] Remove request, id: %s", req.ID)

	// 查找完整的网络 ID
	networks, err := dockerClient.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("获取网络列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	var networkID string
	var networkName string
	for _, n := range networks {
		shortID := n.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		if strings.HasPrefix(n.ID, req.ID) || shortID == req.ID || n.Name == req.ID {
			networkID = n.ID
			networkName = n.Name
			break
		}
	}

	if networkID == "" {
		http.Error(w, "网络不存在", http.StatusNotFound)
		return
	}

	err = dockerClient.NetworkRemove(context.Background(), networkID)
	if err != nil {
		log.Printf("[Network] Remove failed, name: %s, error: %v", networkName, err)
		if strings.Contains(err.Error(), "has active endpoints") {
			http.Error(w, "网络正在被容器使用，请先断开连接", http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("删除网络失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[Network] Removed successfully, name: %s", networkName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 获取网络详情
func handleNetworkInspect(w http.ResponseWriter, r *http.Request) {
	networkID := r.URL.Query().Get("id")
	if networkID == "" {
		http.Error(w, "网络 ID 不能为空", http.StatusBadRequest)
		return
	}

	network, err := dockerClient.NetworkInspect(context.Background(), networkID, types.NetworkInspectOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("获取网络详情失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 获取连接的容器
	containers := make([]map[string]string, 0)
	for id, endpoint := range network.Containers {
		shortID := id
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		containers = append(containers, map[string]string{
			"id":   shortID,
			"name": endpoint.Name,
			"ipv4": endpoint.IPv4Address,
			"ipv6": endpoint.IPv6Address,
			"mac":  endpoint.MacAddress,
		})
	}

	result := map[string]interface{}{
		"id":         network.ID,
		"name":       network.Name,
		"driver":     network.Driver,
		"scope":      network.Scope,
		"internal":   network.Internal,
		"attachable": network.Attachable,
		"ingress":    network.Ingress,
		"ipam":       network.IPAM,
		"options":    network.Options,
		"labels":     network.Labels,
		"containers": containers,
		"created":    network.Created.Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// 连接容器到网络
func handleNetworkConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NetworkID   string `json:"network_id"`
		ContainerID string `json:"container_id"`
		IPv4        string `json:"ipv4"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	endpointConfig := &network.EndpointSettings{}
	if req.IPv4 != "" {
		endpointConfig.IPAMConfig = &network.EndpointIPAMConfig{
			IPv4Address: req.IPv4,
		}
	}

	err := dockerClient.NetworkConnect(context.Background(), req.NetworkID, req.ContainerID, endpointConfig)
	if err != nil {
		http.Error(w, fmt.Sprintf("连接失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 断开容器与网络的连接
func handleNetworkDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NetworkID   string `json:"network_id"`
		ContainerID string `json:"container_id"`
		Force       bool   `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	err := dockerClient.NetworkDisconnect(context.Background(), req.NetworkID, req.ContainerID, req.Force)
	if err != nil {
		http.Error(w, fmt.Sprintf("断开连接失败: %v", err), http.StatusInternalServerError)
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
	http.HandleFunc("/api/containers/run", authMiddleware(handleContainerRun))
	http.HandleFunc("/api/containers/logs", authMiddleware(handleContainerLogs)) // 日志流不限制超时
	http.HandleFunc("/api/images", authOrNodeAuthMiddleware(handleImages)) // 支持用户认证或节点认证
	http.HandleFunc("/api/images/remove", authMiddleware(handleImageRemove))
	http.HandleFunc("/api/images/build", authMiddleware(handleImageBuild))
	
	// 网络管理 API
	http.HandleFunc("/api/networks", authMiddleware(handleNetworks))
	http.HandleFunc("/api/networks/create", authMiddleware(handleNetworkCreate))
	http.HandleFunc("/api/networks/remove", authMiddleware(handleNetworkRemove))
	http.HandleFunc("/api/networks/inspect", authMiddleware(handleNetworkInspect))
	http.HandleFunc("/api/networks/connect", authMiddleware(handleNetworkConnect))
	http.HandleFunc("/api/networks/disconnect", authMiddleware(handleNetworkDisconnect))
	
	// 容器终端和文件管理 API
	http.HandleFunc("/api/containers/exec", authMiddleware(handleContainerExec))
	http.HandleFunc("/api/containers/files", authMiddleware(handleContainerFilesList))
	http.HandleFunc("/api/containers/files/mkdir", authMiddleware(handleContainerFileMkdir))
	http.HandleFunc("/api/containers/files/delete", authMiddleware(handleContainerFileDelete))
	http.HandleFunc("/api/containers/files/upload", authMiddleware(handleContainerFileUpload))
	http.HandleFunc("/api/containers/files/download", authMiddleware(handleContainerFileDownload))
	http.HandleFunc("/api/containers/files/read", authMiddleware(handleContainerFileRead))
	http.HandleFunc("/api/containers/files/write", authMiddleware(handleContainerFileWrite))
	http.HandleFunc("/api/containers/inspect", authMiddleware(handleContainerInspect))
	http.HandleFunc("/api/containers/update", authMiddleware(handleContainerUpdate))
	http.HandleFunc("/api/containers/rename", authMiddleware(handleContainerRename))
	http.HandleFunc("/api/containers/recreate", authMiddleware(handleContainerRecreate))
	
	// Compose 管理 API
	initCompose()
	http.HandleFunc("/api/compose/list", authMiddleware(handleComposeList))
	http.HandleFunc("/api/compose/create", authMiddleware(handleComposeCreate))
	http.HandleFunc("/api/compose/file", authMiddleware(handleComposeGetFile))
	http.HandleFunc("/api/compose/save", authMiddleware(handleComposeSaveFile))
	http.HandleFunc("/api/compose/action", authMiddleware(handleComposeAction))
	http.HandleFunc("/api/compose/status", authMiddleware(handleComposeStatus))
	http.HandleFunc("/api/compose/delete", authMiddleware(handleComposeDelete))

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

