package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"

	"rabbit-panel/model"
	"rabbit-panel/repository"
	"rabbit-panel/tool"
)

// AgentService AI 智能体服务
type AgentService struct {
	sqliteRepo  *repository.SQLiteRepository
	dockerRepo  repository.IDockerRepository
	registry    *RegistryService

	config      AgentConfig
	configMutex sync.RWMutex
	configPath  string
}

// AgentConfig AI 配置
type AgentConfig struct {
	APIURL  string `json:"api_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
}

// NewAgentService 创建 AI 服务
func NewAgentService(sr *repository.SQLiteRepository, dr repository.IDockerRepository) *AgentService {
	service := &AgentService{
		sqliteRepo: sr,
		dockerRepo:  dr,
		configPath: "./data/agent.json",
	}
	if err := service.loadConfig(); err != nil {
		log.Printf("[Agent] load config failed: %v", err)
	}
	return service
}

// UseRegistry 让拉镜像时使用已保存的仓库账号。
func (s *AgentService) UseRegistry(registry *RegistryService) {
	s.registry = registry
}

// GetConfig 获取配置
func (s *AgentService) GetConfig() AgentConfig {
	s.configMutex.RLock()
	defer s.configMutex.RUnlock()
	cfg := s.config
	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-3.5-turbo"
	}
	return cfg
}

// SaveConfig 保存配置
func (s *AgentService) SaveConfig(cfg AgentConfig) error {
	s.configMutex.Lock()
	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-3.5-turbo"
	}
	if strings.TrimSpace(cfg.APIKey) == "" || cfg.APIKey == "****" || strings.Contains(cfg.APIKey, "****") {
		cfg.APIKey = s.config.APIKey
	}
	s.config = cfg
	s.configMutex.Unlock()
	return s.persistConfig()
}

// GetChatHistory 获取聊天历史
func (s *AgentService) GetChatHistory(limit int) ([]repository.ChatHistoryRecord, error) {
	return s.sqliteRepo.GetChatHistory(limit)
}

// SaveChatMessage 保存聊天消息
func (s *AgentService) SaveChatMessage(role, content string) error {
	return s.sqliteRepo.SaveChatMessage(role, content)
}

// CleanupOldMessages 清理旧消息
func (s *AgentService) CleanupOldMessages(olderThan time.Duration) (int64, error) {
	if olderThan <= 0 {
		olderThan = 7 * 24 * time.Hour
	}
	return s.sqliteRepo.CleanupOldMessages(olderThan)
}

func (s *AgentService) ClearChatHistory() error {
	return s.sqliteRepo.ClearChatHistory()
}

func (s *AgentService) StreamChat(ctx context.Context, req model.AgentChatRequest, onChunk func(string) error) error {
	cfg := s.GetConfig()
	if !cfg.Enabled {
		return fmt.Errorf("agent is disabled")
	}
	if strings.TrimSpace(cfg.APIURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("agent config is incomplete")
	}

	messages := make([]model.ChatMessage, 0, len(req.History)+2)
	messages = append(messages, model.ChatMessage{Role: "system", Content: systemPrompt})
	messages = append(messages, req.History...)
	messages = append(messages, model.ChatMessage{Role: "user", Content: req.Message})

	for step := 0; step < 3; step++ {
		responseText, err := s.callLLMStream(ctx, cfg, messages, onChunk)
		if err != nil {
			return err
		}
		messages = append(messages, model.ChatMessage{Role: "assistant", Content: responseText})

		toolResults, err := s.executeToolCalls(ctx, responseText, onChunk)
		if err != nil {
			return err
		}
		if len(toolResults) == 0 {
			return nil
		}
		messages = append(messages, model.ChatMessage{
			Role: "user",
			Content: fmt.Sprintf("[SYSTEM] 所有工具执行完成:\n%s\n如无需继续执行工具，请直接总结结果。",
				strings.Join(toolResults, "\n")),
		})
	}

	return nil
}

func (s *AgentService) loadConfig() error {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	if err := os.MkdirAll("./data", 0755); err != nil {
		return err
	}

	file, err := os.Open(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.config = AgentConfig{
				APIURL:  "https://api.openai.com/v1",
				Model:   "gpt-3.5-turbo",
				Enabled: false,
			}
			return nil
		}
		return err
	}
	defer file.Close()

	var cfg AgentConfig
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return err
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-3.5-turbo"
	}
	s.config = cfg
	return nil
}

func (s *AgentService) persistConfig() error {
	s.configMutex.RLock()
	cfg := s.config
	s.configMutex.RUnlock()

	if err := os.MkdirAll("./data", 0755); err != nil {
		return err
	}
	file, err := os.Create(s.configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

func (s *AgentService) MaskedConfig() AgentConfig {
	cfg := s.GetConfig()
	if len(cfg.APIKey) > 8 {
		cfg.APIKey = cfg.APIKey[:4] + "****" + cfg.APIKey[len(cfg.APIKey)-4:]
	} else if cfg.APIKey != "" {
		cfg.APIKey = "****"
	}
	return cfg
}

func (s *AgentService) callLLMStream(ctx context.Context, cfg AgentConfig, messages []model.ChatMessage, onChunk func(string) error) (string, error) {
	apiURL := strings.TrimRight(cfg.APIURL, "/") + "/chat/completions"
	reqBody := model.ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   true,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("agent api request failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	reader := bufio.NewReader(resp.Body)
	var fullResponse strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk model.ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		content := chunk.Choices[0].Delta.Content
		if content == "" {
			continue
		}
		fullResponse.WriteString(content)
		if err := onChunk(content); err != nil {
			return "", err
		}
	}

	return fullResponse.String(), nil
}

func (s *AgentService) executeToolCalls(ctx context.Context, responseText string, onChunk func(string) error) ([]string, error) {
	var toolResults []string
	remaining := responseText
	for {
		startIdx := strings.Index(remaining, "[[TOOL:")
		if startIdx == -1 {
			break
		}
		remaining = remaining[startIdx+7:]
		endIdx := strings.Index(remaining, "]]")
		if endIdx == -1 {
			break
		}
		toolCmd := strings.TrimSpace(remaining[:endIdx])
		remaining = remaining[endIdx+2:]

		toolResult := s.executeTool(ctx, toolCmd)
		toolResults = append(toolResults, fmt.Sprintf("工具 %s 执行结果: %s", toolCmd, toolResult))
		displayOutput := fmt.Sprintf("\n\n> 🛠️ **系统正在执行**: `%s`\n\n%s\n", toolCmd, toolResult)
		if err := onChunk(displayOutput); err != nil {
			return nil, err
		}
	}
	return toolResults, nil
}

func (s *AgentService) executeTool(ctx context.Context, command string) string {
	idx := strings.Index(command, "(")
	if idx == -1 || !strings.HasSuffix(command, ")") {
		return "Invalid command format"
	}
	name := strings.TrimSpace(command[:idx])
	argsStr := command[idx+1 : len(command)-1]
	args := strings.Split(argsStr, ",")

	cleanArgs := make([]string, 0, len(args))
	for _, a := range args {
		trimmed := strings.TrimSpace(a)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			trimmed = strings.TrimSpace(parts[1])
		}
		trimmed = strings.Trim(trimmed, "\"'")
		cleanArgs = append(cleanArgs, trimmed)
	}

	arg1 := ""
	if len(cleanArgs) > 0 {
		arg1 = cleanArgs[0]
	}

	switch name {
	case "list_containers":
		containers, err := s.dockerRepo.ContainerList(ctx, types.ContainerListOptions{All: true})
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("| ID | 名称 | 状态 |\n")
		sb.WriteString("| --- | --- | --- |\n")
		for _, c := range containers {
			cName := c.ID[:12]
			if len(c.Names) > 0 {
				cName = strings.TrimPrefix(c.Names[0], "/")
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", c.ID[:12], cName, c.Status))
		}
		return sb.String()

	case "start_container":
		if arg1 == "" {
			return "Missing container ID"
		}
		if err := s.dockerRepo.ContainerStart(ctx, arg1, container.StartOptions{}); err != nil {
			return fmt.Sprintf("Error starting: %v", err)
		}
		return fmt.Sprintf("✅ Container %s started.", arg1)

	case "stop_container":
		if arg1 == "" {
			return "Missing container ID"
		}
		timeout := 10
		if err := s.dockerRepo.ContainerStop(ctx, arg1, &timeout); err != nil {
			return fmt.Sprintf("Error stopping: %v", err)
		}
		return fmt.Sprintf("✅ Container %s stopped.", arg1)

	case "restart_container":
		if arg1 == "" {
			return "Missing container ID"
		}
		timeout := 10
		if err := s.dockerRepo.ContainerRestart(ctx, arg1, &timeout); err != nil {
			return fmt.Sprintf("Error restarting: %v", err)
		}
		return fmt.Sprintf("✅ Container %s restarted.", arg1)

	case "get_container_logs":
		if arg1 == "" {
			return "Missing container ID"
		}
		out, err := s.dockerRepo.ContainerLogs(ctx, arg1, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "50",
		})
		if err != nil {
			return fmt.Sprintf("Error reading logs: %v", err)
		}
		defer out.Close()
		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, out)
		return "```\n" + buf.String() + "\n```"

	case "inspect_container":
		if arg1 == "" {
			return "Missing container ID"
		}
		info, err := s.dockerRepo.ContainerInspect(ctx, arg1)
		if err != nil {
			return fmt.Sprintf("Error inspecting: %v", err)
		}
		return fmt.Sprintf("| 属性 | 值 |\n| --- | --- |\n| 名称 | %s |\n| 状态 | %s |\n| IP地址 | %s |\n| 镜像 | %s |",
			strings.TrimPrefix(info.Name, "/"), info.State.Status, info.NetworkSettings.IPAddress, info.Config.Image)

	case "system_status":
		cpu, _ := tool.GetCPUUsage()
		mem, _ := tool.GetMemoryUsage()
		disk, _ := tool.GetDiskUsage()
		return fmt.Sprintf("| 指标 | 使用率 |\n| --- | --- |\n| CPU | %.1f%% |\n| 内存 | %.1f%% |\n| 磁盘 | %.1f%% |", cpu, mem.Usage, disk.Usage)

	case "delete_container":
		if arg1 == "" {
			return "Missing container ID"
		}
		if err := s.dockerRepo.ContainerRemove(ctx, arg1, container.RemoveOptions{Force: true}); err != nil {
			return fmt.Sprintf("Error deleting: %v", err)
		}
		return fmt.Sprintf("✅ Container %s deleted.", arg1)

	case "run_container":
		if arg1 == "" {
			return "Missing image name"
		}
		cmdArgs := []string{"run", "-d"}
		if len(cleanArgs) > 1 && cleanArgs[1] != "" {
			cmdArgs = append(cmdArgs, "--name", cleanArgs[1])
		}
		for i := 2; i < len(cleanArgs); i++ {
			opt := cleanArgs[i]
			switch {
			case strings.HasPrefix(opt, "-p"), strings.HasPrefix(opt, "-e"), strings.HasPrefix(opt, "-v"), strings.HasPrefix(opt, "--restart"):
				cmdArgs = append(cmdArgs, splitToolOption(opt)...)
			case strings.Contains(opt, ":"):
				cmdArgs = append(cmdArgs, "-p", opt)
			}
		}
		cmdArgs = append(cmdArgs, arg1)
		cmd := exec.Command("docker", cmdArgs...)
		output, err := cmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(output))
		if err != nil {
			return fmt.Sprintf("❌ Error: %v\n%s", err, outputStr)
		}
		containerID := outputStr
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}
		displayName := containerID
		if len(cleanArgs) > 1 && cleanArgs[1] != "" {
			displayName = cleanArgs[1]
		}
		return fmt.Sprintf("✅ Container %s created and started (ID: %s)", displayName, containerID)

	case "list_images":
		images, err := s.dockerRepo.ImageList(ctx, types.ImageListOptions{})
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("| ID | 名称 | 大小 |\n")
		sb.WriteString("| --- | --- | --- |\n")
		for _, img := range images {
			name := "<none>"
			if len(img.RepoTags) > 0 {
				name = img.RepoTags[0]
			}
			size := fmt.Sprintf("%.1f MB", float64(img.Size)/1024/1024)
			shortID := strings.TrimPrefix(img.ID, "sha256:")
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", shortID, name, size))
		}
		return sb.String()

	case "pull_image":
		if arg1 == "" {
			return "Missing image name"
		}
		opts := types.ImagePullOptions{}
		if s.registry != nil {
			opts = s.registry.PullOptions(arg1)
		}
		out, err := s.dockerRepo.ImagePull(ctx, arg1, opts)
		if err != nil {
			return fmt.Sprintf("❌ Error pulling: %v", err)
		}
		defer out.Close()
		_, _ = io.Copy(io.Discard, out)
		return fmt.Sprintf("✅ Image %s pulled successfully.", arg1)

	case "delete_image":
		if arg1 == "" {
			return "Missing image ID"
		}
		if _, err := s.dockerRepo.ImageRemove(ctx, arg1, types.ImageRemoveOptions{Force: true}); err != nil {
			return fmt.Sprintf("Error deleting: %v", err)
		}
		return fmt.Sprintf("✅ Image %s deleted.", arg1)

	case "list_networks":
		networks, err := s.dockerRepo.NetworkList(ctx, types.NetworkListOptions{})
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("| ID | 名称 | 驱动 | 范围 |\n")
		sb.WriteString("| --- | --- | --- | --- |\n")
		for _, n := range networks {
			shortID := n.ID
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", shortID, n.Name, n.Driver, n.Scope))
		}
		return sb.String()

	case "create_network":
		if arg1 == "" {
			return "Missing network name"
		}
		driver := "bridge"
		if len(cleanArgs) > 1 && cleanArgs[1] != "" {
			driver = cleanArgs[1]
		}
		resp, err := s.dockerRepo.NetworkCreate(ctx, arg1, types.NetworkCreate{Driver: driver})
		if err != nil {
			return fmt.Sprintf("Error creating: %v", err)
		}
		shortID := resp.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		return fmt.Sprintf("✅ Network %s created (ID: %s)", arg1, shortID)

	case "delete_network":
		if arg1 == "" {
			return "Missing network ID"
		}
		if err := s.dockerRepo.NetworkRemove(ctx, arg1); err != nil {
			return fmt.Sprintf("Error deleting: %v", err)
		}
		return fmt.Sprintf("✅ Network %s deleted.", arg1)

	case "inspect_network":
		if arg1 == "" {
			return "Missing network ID"
		}
		info, err := s.dockerRepo.NetworkInspect(ctx, arg1, types.NetworkInspectOptions{})
		if err != nil {
			return fmt.Sprintf("Error inspecting: %v", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**网络**: %s\n\n", info.Name))
		sb.WriteString("| 属性 | 值 |\n| --- | --- |\n")
		shortID := info.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		sb.WriteString(fmt.Sprintf("| ID | %s |\n", shortID))
		sb.WriteString(fmt.Sprintf("| 驱动 | %s |\n", info.Driver))
		sb.WriteString(fmt.Sprintf("| 范围 | %s |\n", info.Scope))
		sb.WriteString(fmt.Sprintf("| 容器数 | %d |\n", len(info.Containers)))
		return sb.String()

	case "list_volumes":
		vols, err := s.dockerRepo.VolumeList(ctx, volume.ListOptions{})
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("| 名称 | 驱动 |\n")
		sb.WriteString("| --- | --- |\n")
		for _, v := range vols.Volumes {
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", v.Name, v.Driver))
		}
		return sb.String()

	case "create_volume":
		if arg1 == "" {
			return "Missing volume name"
		}
		vol, err := s.dockerRepo.VolumeCreate(ctx, volume.CreateOptions{Name: arg1})
		if err != nil {
			return fmt.Sprintf("Error creating: %v", err)
		}
		return fmt.Sprintf("✅ Volume %s created.", vol.Name)

	case "delete_volume":
		if arg1 == "" {
			return "Missing volume name"
		}
		if err := s.dockerRepo.VolumeRemove(ctx, arg1, true); err != nil {
			return fmt.Sprintf("Error deleting: %v", err)
		}
		return fmt.Sprintf("✅ Volume %s deleted.", arg1)

	case "list_compose_projects":
		entries, err := os.ReadDir("./compose_projects")
		if err != nil {
			return fmt.Sprintf("Error reading compose dir: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("| 项目名 | 状态 |\n")
		sb.WriteString("| --- | --- |\n")
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			status := "unknown"
			projectDir := filepath.Join("./compose_projects", entry.Name())
			cmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "ps", "--format", "json")
			cmd.Dir = projectDir
			output, err := cmd.Output()
			if err == nil {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				running := 0
				total := 0
				for _, line := range lines {
					if strings.TrimSpace(line) == "" {
						continue
					}
					total++
					var item struct {
						State string `json:"State"`
					}
					if json.Unmarshal([]byte(line), &item) == nil && item.State == "running" {
						running++
					}
				}
				switch {
				case total == 0:
					status = "stopped"
				case running == total:
					status = "running"
				case running > 0:
					status = "partial"
				default:
					status = "stopped"
				}
			}
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", entry.Name(), status))
		}
		return sb.String()

	case "compose_up", "compose_down", "compose_restart", "compose_status":
		if arg1 == "" {
			return "Missing project name"
		}
		projectDir := filepath.Join("./compose_projects", arg1)
		var cmd *exec.Cmd
		switch name {
		case "compose_up":
			cmd = exec.Command("docker", "compose", "-f", "docker-compose.yml", "up", "-d")
		case "compose_down":
			cmd = exec.Command("docker", "compose", "-f", "docker-compose.yml", "down")
		case "compose_restart":
			cmd = exec.Command("docker", "compose", "-f", "docker-compose.yml", "restart")
		case "compose_status":
			cmd = exec.Command("docker", "compose", "-f", "docker-compose.yml", "ps", "--format", "table {{.Name}}\t{{.Status}}")
		}
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Error: %v\n%s", err, strings.TrimSpace(string(output)))
		}
		switch name {
		case "compose_up":
			return fmt.Sprintf("✅ Compose project %s started.\n```\n%s\n```", arg1, strings.TrimSpace(string(output)))
		case "compose_down":
			return fmt.Sprintf("✅ Compose project %s stopped.\n```\n%s\n```", arg1, strings.TrimSpace(string(output)))
		case "compose_restart":
			return fmt.Sprintf("✅ Compose project %s restarted.\n```\n%s\n```", arg1, strings.TrimSpace(string(output)))
		default:
			return fmt.Sprintf("**项目 %s 状态:**\n```\n%s\n```", arg1, strings.TrimSpace(string(output)))
		}

	case "prune_containers":
		cmd := exec.Command("docker", "container", "prune", "-f")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Error: %v\n%s", err, strings.TrimSpace(string(output)))
		}
		return fmt.Sprintf("✅ 已清理已停止容器。\n```\n%s\n```", strings.TrimSpace(string(output)))

	case "prune_images":
		cmd := exec.Command("docker", "image", "prune", "-f")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Error: %v\n%s", err, strings.TrimSpace(string(output)))
		}
		return fmt.Sprintf("✅ 已清理未使用镜像。\n```\n%s\n```", strings.TrimSpace(string(output)))

	default:
		return "Unknown tool: " + name
	}
}

const systemPrompt = `你是一个专业的 Rabbit Panel 运维助手。
你的职责是协助用户管理服务器、Docker 容器、镜像、网络、存储卷和 Compose 项目。

回复规范:
1. 尽量使用 Markdown 表格、列表和代码块来展示数据。
2. 当你需要执行操作时，请使用 [[TOOL: function(args)]] 格式。
3. 在执行停止、重启或删除等关键操作前，请先征得用户确认。
4. 仅回答用户提出的问题，不要进行未经请求的分析。
5. 使用中文回复，保持精简。

可用工具:
1. list_containers()
2. start_container(id)
3. stop_container(id)
4. restart_container(id)
5. get_container_logs(id)
6. inspect_container(id)
7. delete_container(id)
8. run_container(image, name, options...)
9. list_images()
10. pull_image(name)
11. delete_image(id)
12. list_networks()
13. create_network(name, driver)
14. delete_network(id)
15. inspect_network(id)
16. list_volumes()
17. create_volume(name)
18. delete_volume(name)
19. list_compose_projects()
20. compose_up(project)
21. compose_down(project)
22. compose_restart(project)
23. compose_status(project)
24. system_status()
25. prune_containers()
26. prune_images()`

func splitToolOption(opt string) []string {
	opt = strings.TrimSpace(opt)
	if strings.Contains(opt, "=") {
		parts := strings.SplitN(opt, "=", 2)
		return []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}
	}
	if strings.Contains(opt, " ") {
		parts := strings.SplitN(opt, " ", 2)
		return []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}
	}
	return []string{opt}
}
