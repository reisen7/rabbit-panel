package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/websocket"
)

// ========== 容器终端执行命令 ==========

// 执行命令请求
type ExecRequest struct {
	ContainerID string   `json:"container_id"`
	Command     []string `json:"command"` // 如 ["ls", "-la"] 或 ["sh", "-c", "echo hello"]
}

// 执行命令响应
type ExecResponse struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// 执行容器命令
func handleContainerExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	if req.ContainerID == "" {
		http.Error(w, "容器ID不能为空", http.StatusBadRequest)
		return
	}

	if len(req.Command) == 0 {
		http.Error(w, "命令不能为空", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建 exec 实例
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          req.Command,
	}

	execID, err := dockerClient.ContainerExecCreate(ctx, req.ContainerID, execConfig)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExecResponse{Error: fmt.Sprintf("创建执行实例失败: %v", err)})
		return
	}

	// 附加到 exec 实例
	resp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExecResponse{Error: fmt.Sprintf("附加执行实例失败: %v", err)})
		return
	}
	defer resp.Close()

	// 读取输出
	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil && err != io.EOF {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExecResponse{Error: fmt.Sprintf("读取输出失败: %v", err)})
		return
	}

	// 获取退出码
	inspectResp, err := dockerClient.ContainerExecInspect(ctx, execID.ID)
	exitCode := 0
	if err == nil {
		exitCode = inspectResp.ExitCode
	}

	// 合并输出
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ExecResponse{
		Output:   output,
		ExitCode: exitCode,
	})
}

// ========== 容器文件管理 ==========

// 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// 列出目录内容
func handleContainerFilesList(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("id")
	dirPath := r.URL.Query().Get("path")

	if containerID == "" {
		http.Error(w, "容器ID不能为空", http.StatusBadRequest)
		return
	}

	if dirPath == "" {
		dirPath = "/"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 使用 ls 命令列出目录（不使用 --time-style，兼容 BusyBox）
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"ls", "-la", dirPath},
	}

	execID, err := dockerClient.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		http.Error(w, fmt.Sprintf("执行命令失败: %v", err), http.StatusInternalServerError)
		return
	}

	resp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		http.Error(w, fmt.Sprintf("附加执行失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	stdcopy.StdCopy(&stdout, &stderr, resp.Reader)

	// 检查错误输出
	stderrStr := stderr.String()
	if stderrStr != "" && (strings.Contains(stderrStr, "No such file") || strings.Contains(stderrStr, "not found")) {
		http.Error(w, "目录不存在", http.StatusNotFound)
		return
	}

	// 解析 ls 输出
	files := parseLsOutput(stdout.String(), dirPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// 解析 ls -la 输出（兼容 GNU ls 和 BusyBox ls）
func parseLsOutput(output string, basePath string) []FileInfo {
	files := make([]FileInfo, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		// ls -la 输出格式：
		// GNU:     drwxr-xr-x 2 root root 4096 Jan  1 12:00 dirname
		// BusyBox: drwxr-xr-x    2 root     root          4096 Jan  1 12:00 dirname
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		mode := fields[0]
		
		// 找到文件名（最后一个或多个字段）
		// 时间格式可能是 "Jan 1 12:00" 或 "2024-01-01 12:00"
		var name string
		var modTime string
		var size int64
		
		// 尝试解析大小（通常在第4或第5个字段）
		for i := 3; i < len(fields) && i < 6; i++ {
			if n, err := fmt.Sscanf(fields[i], "%d", &size); n == 1 && err == nil {
				// 找到大小字段，后面是时间和文件名
				// 时间通常占 3 个字段（如 "Jan 1 12:00"）或 2 个字段（如 "2024-01-01 12:00"）
				remaining := fields[i+1:]
				if len(remaining) >= 4 {
					// 可能是 "Jan 1 12:00 filename" 或 "Jan 1 2024 filename"
					modTime = strings.Join(remaining[:3], " ")
					name = strings.Join(remaining[3:], " ")
				} else if len(remaining) >= 3 {
					modTime = strings.Join(remaining[:2], " ")
					name = strings.Join(remaining[2:], " ")
				} else if len(remaining) >= 2 {
					modTime = remaining[0]
					name = strings.Join(remaining[1:], " ")
				} else if len(remaining) == 1 {
					name = remaining[0]
				}
				break
			}
		}

		// 跳过无效行
		if name == "" || name == "." || name == ".." {
			continue
		}

		isDir := strings.HasPrefix(mode, "d")
		filePath := path.Join(basePath, name)

		files = append(files, FileInfo{
			Name:    name,
			Path:    filePath,
			Size:    size,
			Mode:    mode,
			ModTime: modTime,
			IsDir:   isDir,
		})
	}

	return files
}

// 创建目录
func handleContainerFileMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execConfig := types.ExecConfig{
		AttachStderr: true,
		Cmd:          []string{"mkdir", "-p", req.Path},
	}

	execID, err := dockerClient.ContainerExecCreate(ctx, req.ContainerID, execConfig)
	if err != nil {
		http.Error(w, fmt.Sprintf("创建目录失败: %v", err), http.StatusInternalServerError)
		return
	}

	resp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		http.Error(w, fmt.Sprintf("执行失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Close()

	var stderr bytes.Buffer
	stdcopy.StdCopy(io.Discard, &stderr, resp.Reader)

	if stderr.Len() > 0 {
		http.Error(w, stderr.String(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 删除文件或目录
func handleContainerFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	// 安全检查：禁止删除根目录和关键系统目录
	dangerousPaths := []string{"/", "/bin", "/sbin", "/usr", "/lib", "/etc", "/var", "/root", "/home"}
	cleanPath := path.Clean(req.Path)
	for _, dp := range dangerousPaths {
		if cleanPath == dp {
			http.Error(w, "禁止删除系统关键目录", http.StatusForbidden)
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execConfig := types.ExecConfig{
		AttachStderr: true,
		Cmd:          []string{"rm", "-rf", req.Path},
	}

	execID, err := dockerClient.ContainerExecCreate(ctx, req.ContainerID, execConfig)
	if err != nil {
		http.Error(w, fmt.Sprintf("删除失败: %v", err), http.StatusInternalServerError)
		return
	}

	resp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		http.Error(w, fmt.Sprintf("执行失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Close()

	var stderr bytes.Buffer
	stdcopy.StdCopy(io.Discard, &stderr, resp.Reader)

	if stderr.Len() > 0 {
		http.Error(w, stderr.String(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 上传文件到容器
func handleContainerFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`     // 目标目录
		FileName    string `json:"filename"` // 文件名
		Content     string `json:"content"`  // Base64 编码的文件内容
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	// 解码 Base64 内容
	fileContent, err := base64.StdEncoding.DecodeString(req.Content)
	if err != nil {
		http.Error(w, "文件内容解码失败", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 创建 tar 归档
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name: req.FileName,
		Mode: 0644,
		Size: int64(len(fileContent)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		http.Error(w, fmt.Sprintf("创建归档失败: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := tw.Write(fileContent); err != nil {
		http.Error(w, fmt.Sprintf("写入归档失败: %v", err), http.StatusInternalServerError)
		return
	}

	if err := tw.Close(); err != nil {
		http.Error(w, fmt.Sprintf("关闭归档失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 复制到容器
	err = dockerClient.CopyToContainer(ctx, req.ContainerID, req.Path, &buf, types.CopyToContainerOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("上传失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 从容器下载文件
func handleContainerFileDownload(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("id")
	filePath := r.URL.Query().Get("path")

	if containerID == "" || filePath == "" {
		http.Error(w, "参数不完整", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 从容器复制文件
	reader, stat, err := dockerClient.CopyFromContainer(ctx, containerID, filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("下载失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// 解析 tar 归档
	tr := tar.NewReader(reader)
	hdr, err := tr.Next()
	if err != nil {
		http.Error(w, fmt.Sprintf("读取文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 设置响应头
	fileName := path.Base(filePath)
	if stat.Mode.IsDir() {
		fileName += ".tar"
		w.Header().Set("Content-Type", "application/x-tar")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", hdr.Size))

	// 写入响应
	io.Copy(w, tr)
}

// 读取文件内容（用于编辑）
func handleContainerFileRead(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("id")
	filePath := r.URL.Query().Get("path")

	if containerID == "" || filePath == "" {
		http.Error(w, "参数不完整", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 使用 cat 命令读取文件（限制大小）
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"head", "-c", "1048576", filePath}, // 限制 1MB
	}

	execID, err := dockerClient.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		http.Error(w, fmt.Sprintf("读取失败: %v", err), http.StatusInternalServerError)
		return
	}

	resp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		http.Error(w, fmt.Sprintf("执行失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	stdcopy.StdCopy(&stdout, &stderr, resp.Reader)

	if stderr.Len() > 0 && strings.Contains(stderr.String(), "No such file") {
		http.Error(w, "文件不存在", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"content": stdout.String(),
	})
}

// 写入文件内容
func handleContainerFileWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建 tar 归档
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	fileName := path.Base(req.Path)
	dirPath := path.Dir(req.Path)

	hdr := &tar.Header{
		Name: fileName,
		Mode: 0644,
		Size: int64(len(req.Content)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		http.Error(w, fmt.Sprintf("创建归档失败: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := tw.Write([]byte(req.Content)); err != nil {
		http.Error(w, fmt.Sprintf("写入归档失败: %v", err), http.StatusInternalServerError)
		return
	}

	if err := tw.Close(); err != nil {
		http.Error(w, fmt.Sprintf("关闭归档失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 复制到容器
	err := dockerClient.CopyToContainer(ctx, req.ContainerID, dirPath, &buf, types.CopyToContainerOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("写入失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// ========== 容器配置修改 ==========

// 获取容器详细配置
func handleContainerInspect(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("id")
	if containerID == "" {
		http.Error(w, "容器ID不能为空", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	info, err := dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取容器信息失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 格式化端口映射
	ports := []map[string]string{}
	for containerPort, bindings := range info.HostConfig.PortBindings {
		for _, binding := range bindings {
			ports = append(ports, map[string]string{
				"host":      binding.HostPort,
				"container": string(containerPort),
				"hostIP":    binding.HostIP,
			})
		}
	}

	// 格式化数据卷
	volumes := []map[string]string{}
	for _, bind := range info.HostConfig.Binds {
		parts := strings.SplitN(bind, ":", 3)
		vol := map[string]string{"host": "", "container": "", "mode": "rw"}
		if len(parts) >= 1 {
			vol["host"] = parts[0]
		}
		if len(parts) >= 2 {
			vol["container"] = parts[1]
		}
		if len(parts) >= 3 {
			vol["mode"] = parts[2]
		}
		volumes = append(volumes, vol)
	}

	// 格式化环境变量
	envs := []map[string]string{}
	for _, env := range info.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envs = append(envs, map[string]string{
				"key":   parts[0],
				"value": parts[1],
			})
		}
	}

	// 提取完整配置信息
	config := map[string]interface{}{
		// 基本信息
		"id":        info.ID[:12],
		"fullId":    info.ID,
		"name":      strings.TrimPrefix(info.Name, "/"),
		"image":     info.Config.Image,
		"imageId":   info.Image,
		"created":   info.Created,
		"started":   info.State.StartedAt,
		"finished":  info.State.FinishedAt,
		"state":     info.State.Status,
		"running":   info.State.Running,
		"paused":    info.State.Paused,
		"pid":       info.State.Pid,
		"exitCode":  info.State.ExitCode,
		"platform":  info.Platform,

		// 网络配置
		"hostname":    info.Config.Hostname,
		"domainname":  info.Config.Domainname,
		"networkMode": string(info.HostConfig.NetworkMode),
		"ports":       ports,
		"dns":         info.HostConfig.DNS,
		"dnsSearch":   info.HostConfig.DNSSearch,
		"extraHosts":  info.HostConfig.ExtraHosts,
		"macAddress":  info.NetworkSettings.MacAddress,
		"ipAddress":   info.NetworkSettings.IPAddress,
		"gateway":     info.NetworkSettings.Gateway,

		// 存储配置
		"volumes":    volumes,
		"workingDir": info.Config.WorkingDir,
		"readOnly":   info.HostConfig.ReadonlyRootfs,

		// 运行配置
		"env":        envs,
		"cmd":        info.Config.Cmd,
		"entrypoint": info.Config.Entrypoint,
		"user":       info.Config.User,
		"tty":        info.Config.Tty,
		"stdin":      info.Config.OpenStdin,

		// 重启策略
		"restart":         string(info.HostConfig.RestartPolicy.Name),
		"restartMaxRetry": info.HostConfig.RestartPolicy.MaximumRetryCount,

		// 资源限制
		"memory":       info.HostConfig.Memory,
		"memorySwap":   info.HostConfig.MemorySwap,
		"memoryRes":    info.HostConfig.MemoryReservation,
		"cpus":         info.HostConfig.NanoCPUs,
		"cpuShares":    info.HostConfig.CPUShares,
		"cpusetCpus":   info.HostConfig.CpusetCpus,
		"cpusetMems":   info.HostConfig.CpusetMems,
		"cpuPeriod":    info.HostConfig.CPUPeriod,
		"cpuQuota":     info.HostConfig.CPUQuota,
		"pidsLimit":    info.HostConfig.PidsLimit,
		"oomKillDisable": info.HostConfig.OomKillDisable,

		// 安全配置
		"privileged":  info.HostConfig.Privileged,
		"capAdd":      info.HostConfig.CapAdd,
		"capDrop":     info.HostConfig.CapDrop,
		"securityOpt": info.HostConfig.SecurityOpt,

		// 标签
		"labels": info.Config.Labels,

		// 日志配置
		"logDriver":  info.HostConfig.LogConfig.Type,
		"logOptions": info.HostConfig.LogConfig.Config,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// 更新容器配置（需要重建容器）
func handleContainerUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContainerID string `json:"container_id"`
		// 可更新的配置
		Memory    int64  `json:"memory"`     // 内存限制（字节）
		CPUs      int64  `json:"cpus"`       // CPU 限制（纳秒）
		Restart   string `json:"restart"`    // 重启策略
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// 构建更新配置
	updateConfig := container.UpdateConfig{}

	if req.Memory > 0 {
		updateConfig.Memory = req.Memory
		// 设置 MemorySwap 为 Memory 的 2 倍（或相同值禁用 swap）
		// -1 表示不限制 swap，设置为 memory 的值表示禁用 swap
		updateConfig.MemorySwap = req.Memory * 2
	} else if req.Memory == 0 {
		// 如果设置为 0，表示取消限制
		// 注意：Docker 不支持在运行时取消内存限制，只能设置新值
	}

	if req.CPUs > 0 {
		updateConfig.NanoCPUs = req.CPUs
	}

	if req.Restart != "" {
		updateConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(req.Restart),
		}
	}

	// 更新容器
	_, err := dockerClient.ContainerUpdate(ctx, req.ContainerID, updateConfig)
	if err != nil {
		http.Error(w, fmt.Sprintf("更新失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 清除缓存
	containersCache.Lock()
	containersCache.lastFetch = time.Time{}
	containersCache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 重命名容器
func handleContainerRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContainerID string `json:"container_id"`
		NewName     string `json:"new_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	err := dockerClient.ContainerRename(ctx, req.ContainerID, req.NewName)
	if err != nil {
		http.Error(w, fmt.Sprintf("重命名失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 清除缓存
	containersCache.Lock()
	containersCache.lastFetch = time.Time{}
	containersCache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}


// ========== 重建容器 ==========

// 重建容器请求
type RecreateContainerRequest struct {
	ContainerID string            `json:"container_id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMapping   `json:"volumes"`
	Env         []EnvVar          `json:"env"`
	Restart     string            `json:"restart"`
	Network     string            `json:"network"`
	Memory      int64             `json:"memory"`
	CPUs        float64           `json:"cpus"`
	Privileged  bool              `json:"privileged"`
	TTY         bool              `json:"tty"`
}

type PortMapping struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}

type VolumeMapping struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// 重建容器处理
func handleContainerRecreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req RecreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// 1. 停止旧容器
	timeout := 10
	stopOptions := container.StopOptions{Timeout: &timeout}
	if err := dockerClient.ContainerStop(ctx, req.ContainerID, stopOptions); err != nil {
		// 忽略已停止的容器错误
		if !strings.Contains(err.Error(), "is not running") {
			http.Error(w, "停止容器失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// 2. 删除旧容器
	removeOptions := container.RemoveOptions{Force: true}
	if err := dockerClient.ContainerRemove(ctx, req.ContainerID, removeOptions); err != nil {
		http.Error(w, "删除容器失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. 构建新容器配置
	// 端口绑定 - 使用 nat.PortMap 和 nat.PortBinding
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}
	for _, p := range req.Ports {
		if p.Host != "" && p.Container != "" {
			containerPort := p.Container
			if !strings.Contains(containerPort, "/") {
				containerPort += "/tcp"
			}
			port := nat.Port(containerPort)
			exposedPorts[port] = struct{}{}
			portBindings[port] = append(portBindings[port], nat.PortBinding{
				HostIP:   "0.0.0.0",
				HostPort: p.Host,
			})
		}
	}

	// 数据卷
	var binds []string
	for _, v := range req.Volumes {
		if v.Host != "" && v.Container != "" {
			binds = append(binds, v.Host+":"+v.Container)
		}
	}

	// 环境变量
	var envList []string
	for _, e := range req.Env {
		if e.Key != "" {
			envList = append(envList, e.Key+"="+e.Value)
		}
	}

	// 创建容器配置
	containerConfig := &container.Config{
		Image:        req.Image,
		Env:          envList,
		Tty:          req.TTY,
		OpenStdin:    req.TTY,
		AttachStdin:  req.TTY,
		AttachStdout: true,
		AttachStderr: true,
		ExposedPorts: exposedPorts,
	}

	// 主机配置
	hostConfig := &container.HostConfig{
		Binds:        binds,
		PortBindings: portBindings,
		NetworkMode:  container.NetworkMode(req.Network),
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode(req.Restart),
		},
		Privileged: req.Privileged,
	}

	// 资源限制
	if req.Memory > 0 {
		hostConfig.Memory = req.Memory * 1024 * 1024
	}
	if req.CPUs > 0 {
		hostConfig.NanoCPUs = int64(req.CPUs * 1e9)
	}

	// 4. 创建新容器
	resp, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, req.Name)
	if err != nil {
		http.Error(w, "创建容器失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. 启动新容器
	if err := dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		http.Error(w, "启动容器失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 清除缓存
	containersCache.Lock()
	containersCache.lastFetch = time.Time{}
	containersCache.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":       "success",
		"container_id": resp.ID,
	})
}

// ========== 容器资源统计 ==========

// 容器资源统计信息
type ContainerStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	CPUCores      int     `json:"cpu_cores"`
	MemoryUsage   int64   `json:"memory_usage"`
	MemoryLimit   int64   `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     int64   `json:"network_rx"`
	NetworkTx     int64   `json:"network_tx"`
	BlockRead     int64   `json:"block_read"`
	BlockWrite    int64   `json:"block_write"`
	PIDs          uint64  `json:"pids"`
}

// 获取容器资源统计
func handleContainerStats(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("id")
	if containerID == "" {
		http.Error(w, "容器ID不能为空", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取容器统计信息（非流式，只获取一次）
	statsResp, err := dockerClient.ContainerStats(ctx, containerID, false)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取统计信息失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer statsResp.Body.Close()

	var stats types.StatsJSON
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		http.Error(w, fmt.Sprintf("解析统计信息失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 计算 CPU 使用率
	cpuPercent := 0.0
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
	}

	// 计算内存使用率
	memoryPercent := 0.0
	if stats.MemoryStats.Limit > 0 {
		memoryPercent = float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100.0
	}

	// 计算网络 IO
	var networkRx, networkTx int64
	for _, netStats := range stats.Networks {
		networkRx += int64(netStats.RxBytes)
		networkTx += int64(netStats.TxBytes)
	}

	// 计算块设备 IO
	var blockRead, blockWrite int64
	for _, bioEntry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch bioEntry.Op {
		case "read", "Read":
			blockRead += int64(bioEntry.Value)
		case "write", "Write":
			blockWrite += int64(bioEntry.Value)
		}
	}

	result := ContainerStats{
		CPUPercent:    cpuPercent,
		CPUCores:      int(stats.CPUStats.OnlineCPUs),
		MemoryUsage:   int64(stats.MemoryStats.Usage),
		MemoryLimit:   int64(stats.MemoryStats.Limit),
		MemoryPercent: memoryPercent,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
		BlockRead:     blockRead,
		BlockWrite:    blockWrite,
		PIDs:          stats.PidsStats.Current,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ========== WebSocket 交互式终端 ==========

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境应该限制）
	},
}

// WebSocket 终端处理
func handleContainerTerminalWS(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("id")
	if containerID == "" {
		http.Error(w, "容器ID不能为空", http.StatusBadRequest)
		return
	}

	// 升级为 WebSocket 连接
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Terminal] WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[Terminal] WebSocket connected, container: %s", containerID)

	ctx := context.Background()

	// 检测容器中可用的 shell
	shell := detectShell(ctx, containerID)
	log.Printf("[Terminal] Using shell: %s for container: %s", shell, containerID)

	// 创建 exec 实例
	execConfig := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{shell},
	}

	execID, err := dockerClient.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		log.Printf("[Terminal] Exec create failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mError: "+err.Error()+"\x1b[0m\r\n"))
		return
	}

	// 附加到 exec 实例
	execAttachConfig := types.ExecStartCheck{
		Tty: true,
	}

	hijackedResp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, execAttachConfig)
	if err != nil {
		log.Printf("[Terminal] Exec attach failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mError: "+err.Error()+"\x1b[0m\r\n"))
		return
	}
	defer hijackedResp.Close()

	// 用于通知 goroutine 退出
	done := make(chan struct{})

	// 从容器读取输出，发送到 WebSocket
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := hijackedResp.Reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("[Terminal] Read from container error: %v", err)
				}
				return
			}
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					log.Printf("[Terminal] WebSocket write error: %v", err)
					return
				}
			}
		}
	}()

	// 从 WebSocket 读取输入，发送到容器
	go func() {
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[Terminal] WebSocket read error: %v", err)
				}
				return
			}

			// 处理终端大小调整消息
			if messageType == websocket.TextMessage && len(message) > 0 && message[0] == '{' {
				var resizeMsg struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if err := json.Unmarshal(message, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
					// 调整终端大小
					dockerClient.ContainerExecResize(ctx, execID.ID, container.ResizeOptions{
						Height: uint(resizeMsg.Rows),
						Width:  uint(resizeMsg.Cols),
					})
					continue
				}
			}

			// 发送输入到容器
			if _, err := hijackedResp.Conn.Write(message); err != nil {
				log.Printf("[Terminal] Write to container error: %v", err)
				return
			}
		}
	}()

	// 等待连接关闭
	<-done
	log.Printf("[Terminal] WebSocket disconnected, container: %s", containerID)
}

// 检测容器中可用的 shell
func detectShell(ctx context.Context, containerID string) string {
	// 按优先级尝试不同的 shell
	shells := []string{"/bin/sh", "/bin/bash", "/bin/ash", "sh"}

	for _, shell := range shells {
		// 直接尝试运行 shell 并立即退出，检查是否可用
		execConfig := types.ExecConfig{
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          []string{shell, "-c", "exit 0"},
		}

		execID, err := dockerClient.ContainerExecCreate(ctx, containerID, execConfig)
		if err != nil {
			continue
		}

		resp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
		if err != nil {
			continue
		}
		resp.Close()

		// 检查退出码
		inspectResp, err := dockerClient.ContainerExecInspect(ctx, execID.ID)
		if err == nil && inspectResp.ExitCode == 0 {
			log.Printf("[Terminal] Detected shell: %s", shell)
			return shell
		}
	}

	// 默认返回 /bin/sh
	return "/bin/sh"
}
