package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const composeBaseDir = "./compose_projects"

type ComposeProject struct {
	Name       string             `json:"name"`
	Status     string             `json:"status"` // "running", "partial", "stopped", "unknown"
	Containers []ComposeContainer `json:"containers,omitempty"`
}

type ComposeContainer struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	State   string `json:"state"`   // "running", "exited", "paused", etc.
	Status  string `json:"status"`  // 详细状态如 "Up 2 hours"
	Ports   string `json:"ports"`
}

type ComposeFileRequest struct {
	Project string `json:"project"`
	Content string `json:"content"`
}

type ComposeActionRequest struct {
	Project string `json:"project"`
	Action  string `json:"action"` // "up", "down", "restart", "pull", "logs"
}

func initCompose() {
	if err := os.MkdirAll(composeBaseDir, 0755); err != nil {
		log.Printf("无法创建 Compose 目录: %v", err)
	}
}

// 获取 Compose 项目列表
func handleComposeList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 禁止缓存
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	entries, err := os.ReadDir(composeBaseDir)
	if err != nil {
		log.Printf("读取 Compose 目录失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	projects := make([]ComposeProject, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			// 简单检查状态：如果目录下有 docker-compose.yml 且 docker compose ps 返回内容则认为运行中
			// 这里为了性能先只返回名字，状态可以在前端单独查询或异步加载
			projects = append(projects, ComposeProject{
				Name:   entry.Name(),
				Status: "unknown",
			})
		}
	}

	log.Printf("获取到 %d 个 Compose 项目", len(projects))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

// 创建新项目
func handleComposeCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "项目名称不能为空", http.StatusBadRequest)
		return
	}

	projectDir := filepath.Join(composeBaseDir, req.Name)
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		http.Error(w, "项目已存在", http.StatusConflict)
		return
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 创建默认 docker-compose.yml
	defaultContent := "version: '3'\nservices:\n  web:\n    image: nginx:alpine\n    ports:\n      - \"8080:80\"\n"
	if err := ioutil.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(defaultContent), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// 获取 Compose 文件内容
func handleComposeGetFile(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		http.Error(w, "Missing project parameter", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(composeBaseDir, project, "docker-compose.yml")
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 尝试 .yaml
			filePath = filepath.Join(composeBaseDir, project, "docker-compose.yaml")
			content, err = ioutil.ReadFile(filePath)
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(content)
}

// 保存 Compose 文件内容
func handleComposeSaveFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ComposeFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(composeBaseDir, req.Project, "docker-compose.yml")
	if err := ioutil.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// 获取 Compose 项目状态（包含容器详情）
func handleComposeStatus(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		http.Error(w, "Missing project parameter", http.StatusBadRequest)
		return
	}

	projectDir := filepath.Join(composeBaseDir, project)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// 使用 docker compose ps --format json 获取容器状态
	cmd := exec.Command("docker", "compose", "ps", "--format", "json", "-a")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		// 可能是没有运行的容器，返回空列表
		result := ComposeProject{
			Name:       project,
			Status:     "stopped",
			Containers: []ComposeContainer{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	// 解析 JSON 输出（每行一个 JSON 对象）
	containers := []ComposeContainer{}
	runningCount := 0
	totalCount := 0

	// docker compose ps --format json 输出每行一个 JSON
	lines := splitLines(string(output))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var containerInfo struct {
			Name    string `json:"Name"`
			Service string `json:"Service"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Ports   string `json:"Ports"`
		}
		if err := json.Unmarshal([]byte(line), &containerInfo); err != nil {
			continue
		}
		totalCount++
		if containerInfo.State == "running" {
			runningCount++
		}
		containers = append(containers, ComposeContainer{
			Name:    containerInfo.Name,
			Service: containerInfo.Service,
			State:   containerInfo.State,
			Status:  containerInfo.Status,
			Ports:   containerInfo.Ports,
		})
	}

	// 计算整体状态
	status := "stopped"
	if totalCount > 0 {
		if runningCount == totalCount {
			status = "running"
		} else if runningCount > 0 {
			status = "partial"
		}
	}

	result := ComposeProject{
		Name:       project,
		Status:     status,
		Containers: containers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// 辅助函数：分割行
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// 执行 Compose 操作
func handleComposeAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ComposeActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	projectDir := filepath.Join(composeBaseDir, req.Project)
	var cmd *exec.Cmd

	switch req.Action {
	case "up":
		cmd = exec.Command("docker", "compose", "up", "-d")
	case "down":
		cmd = exec.Command("docker", "compose", "down")
	case "restart":
		cmd = exec.Command("docker", "compose", "restart")
	case "pull":
		cmd = exec.Command("docker", "compose", "pull")
	case "logs":
		// 日志特殊处理，返回最后 100 行
		cmd = exec.Command("docker", "compose", "logs", "--tail=100")
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}

	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 返回错误信息和输出
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Error: %v\nOutput:\n%s", err, string(output))))
		return
	}

	w.Write(output)
}

// 删除 Compose 项目
func handleComposeDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Project == "" {
		http.Error(w, "项目名称不能为空", http.StatusBadRequest)
		return
	}

	projectDir := filepath.Join(composeBaseDir, req.Project)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		http.Error(w, "项目不存在", http.StatusNotFound)
		return
	}

	// 先尝试停止容器
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = projectDir
	cmd.Run() // 忽略错误，可能本来就没有运行

	// 删除项目目录
	if err := os.RemoveAll(projectDir); err != nil {
		http.Error(w, fmt.Sprintf("删除失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
