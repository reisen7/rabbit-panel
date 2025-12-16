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
	Name   string `json:"name"`
	Status string `json:"status"` // "running", "stopped", "unknown"
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
