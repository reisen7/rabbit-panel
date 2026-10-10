package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"

	"rabbit-panel/tool"
)

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
	name, cleanArgs, nodeHint, ok := parseToolCommand(command)
	if !ok {
		return "Invalid command format"
	}
	if name == "list_nodes" {
		return s.formatNodeList(ctx)
	}
	address, label, err := s.resolveToolNode(ctx, nodeHint)
	if err != nil {
		return "Error: " + err.Error()
	}
	var result string
	if address != "" {
		result = s.executeRemoteTool(ctx, address, name, cleanArgs)
	} else {
		result = s.executeLocalTool(ctx, name, cleanArgs)
	}
	if label == "" {
		return result
	}
	if address == "" {
		label += "（本机）"
	}
	return "节点 " + label + "\n\n" + result
}

func (s *AgentService) executeLocalTool(ctx context.Context, name string, cleanArgs []string) string {
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
func fallbackToolCommand(message string) string {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return ""
	}
	if !strings.Contains(text, "容器") && !strings.Contains(text, "container") && !strings.Contains(text, "docker ps") {
		return ""
	}
	for _, word := range []string{"停止", "重启", "删除", "启动", "日志", "stop", "restart", "delete", "remove", "start", "log"} {
		if strings.Contains(text, word) {
			return ""
		}
	}
	for _, word := range []string{"哪些", "哪个", "有哪", "列出", "列表", "查看", "看看", "运行", "状态", "list", "ps", "running"} {
		if strings.Contains(text, word) {
			return "list_containers()"
		}
	}
	return ""
}

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
