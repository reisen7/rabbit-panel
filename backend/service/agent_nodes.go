package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"rabbit-panel/middleware"
	"rabbit-panel/model"
)

type agentNodeKey struct{}

func (s *AgentService) nodePrompt(selected string) string {
	if s.nodes == nil || s.nodes.GetMode() != model.ModeMaster {
		return ""
	}
	list := sortedNodes(s.nodes.GetAllNodes())
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n当前节点:\n")
	selected = strings.TrimSpace(selected)
	if selected == "" {
		b.WriteString("面板当前没有选中远程节点。未写 node 的工具操作本机主节点。\n")
	} else {
		b.WriteString("面板当前选中的节点 ID 是 " + selected + "。未写 node 的工具操作这台节点。\n")
	}
	b.WriteString("已知节点:\n")
	for _, n := range list {
		b.WriteString(fmt.Sprintf("- 名称 %s，ID %s，%s，%s，地址 %s，容器 %d\n",
			n.Name, n.ID, nodeModeText(n.Mode), nodeStatusText(n.Status), n.Address, n.Containers))
	}
	return b.String()
}

func (s *AgentService) formatNodeList(ctx context.Context) string {
	if s.nodes == nil || s.nodes.GetMode() != model.ModeMaster {
		return "当前进程不是主节点，只能操作本机 Docker。"
	}
	selected, _ := ctx.Value(agentNodeKey{}).(string)
	list := sortedNodes(s.nodes.GetAllNodes())
	var b strings.Builder
	b.WriteString("| 名称 | ID | 角色 | 状态 | 地址 | CPU | 内存 | 容器 |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, n := range list {
		mark := ""
		if selected != "" && (n.ID == selected || strings.EqualFold(n.Name, selected)) {
			mark = "（当前选中）"
		}
		b.WriteString(fmt.Sprintf("| %s%s | %s | %s | %s | %s | %.1f%% | %.1f%% | %d |\n",
			n.Name, mark, n.ID, nodeModeText(n.Mode), nodeStatusText(n.Status), n.Address, n.CPU, n.Memory, n.Containers))
	}
	if len(list) == 0 {
		return "没有已登记的节点。"
	}
	return b.String()
}

func (s *AgentService) withMentionedNode(command, message string) string {
	if command != "list_containers()" {
		return command
	}
	name := s.nodeMentioned(message)
	if name == "" {
		return command
	}
	return "list_containers(node=" + name + ")"
}

func (s *AgentService) nodeMentioned(message string) string {
	if s.nodes == nil || s.nodes.GetMode() != model.ModeMaster {
		return ""
	}
	lower := strings.ToLower(message)
	best := ""
	ambiguous := false
	for _, n := range s.nodes.GetAllNodes() {
		name := strings.TrimSpace(n.Name)
		if len([]rune(name)) < 2 || !strings.Contains(lower, strings.ToLower(name)) {
			continue
		}
		if best == "" || len(name) > len(best) {
			best = name
			ambiguous = false
			continue
		}
		if len(name) == len(best) && !strings.EqualFold(name, best) {
			ambiguous = true
		}
	}
	if ambiguous {
		return ""
	}
	return best
}

func (s *AgentService) resolveToolNode(ctx context.Context, hint string) (string, string, error) {
	if s.nodes == nil {
		return "", "", nil
	}
	hint = strings.TrimSpace(hint)
	if hint == "" {
		if selected, ok := ctx.Value(agentNodeKey{}).(string); ok {
			hint = strings.TrimSpace(selected)
		}
	}
	if hint == "" {
		return "", "", nil
	}
	node, err := s.nodes.FindNode(hint)
	if err != nil {
		return "", "", err
	}
	address, err := s.nodes.ProxyTarget(node.ID)
	if err != nil {
		return "", "", err
	}
	label := strings.TrimSpace(node.Name)
	if label == "" {
		label = node.ID
	}
	return address, label, nil
}

func (s *AgentService) executeRemoteTool(ctx context.Context, address, name string, args []string) string {
	arg1 := ""
	if len(args) > 0 {
		arg1 = args[0]
	}
	switch name {
	case "list_containers":
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/containers", nil, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var containers []model.ContainerInfo
		if err := json.Unmarshal(raw, &containers); err != nil {
			return "Error: " + err.Error()
		}
		var b strings.Builder
		b.WriteString("| ID | 名称 | 状态 |\n| --- | --- | --- |\n")
		for _, c := range containers {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", c.ID, c.Name, c.Status))
		}
		return b.String()
	case "start_container", "stop_container", "restart_container", "delete_container":
		if arg1 == "" {
			return "Missing container ID"
		}
		action := map[string]string{
			"start_container":   "start",
			"stop_container":    "stop",
			"restart_container": "restart",
			"delete_container":  "remove",
		}[name]
		_, err := s.callNode(ctx, address, http.MethodPost, "/api/containers/action", nil, map[string]string{
			"container_id": arg1,
			"action":       action,
		})
		if err != nil {
			return "Error: " + err.Error()
		}
		verb := map[string]string{"start": "started", "stop": "stopped", "restart": "restarted", "remove": "deleted"}[action]
		return fmt.Sprintf("✅ Container %s %s.", arg1, verb)
	case "get_container_logs":
		if arg1 == "" {
			return "Missing container ID"
		}
		q := url.Values{"id": {arg1}, "tail": {"50"}, "follow": {"false"}}
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/containers/logs", q, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		return "```\n" + sseText(string(raw)) + "\n```"
	case "inspect_container":
		if arg1 == "" {
			return "Missing container ID"
		}
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/containers/inspect", url.Values{"id": {arg1}}, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var info struct {
			Name  string `json:"name"`
			State string `json:"state"`
			IP    string `json:"ipAddress"`
			Image string `json:"image"`
		}
		if err := json.Unmarshal(raw, &info); err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("| 属性 | 值 |\n| --- | --- |\n| 名称 | %s |\n| 状态 | %s |\n| IP地址 | %s |\n| 镜像 | %s |", info.Name, info.State, info.IP, info.Image)
	case "run_container":
		body, errText := remoteRunBody(args)
		if errText != "" {
			return errText
		}
		raw, err := s.callNode(ctx, address, http.MethodPost, "/api/containers/run", nil, body)
		if err != nil {
			return "Error: " + err.Error()
		}
		var resp struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &resp)
		id := resp.ID
		if len(id) > 12 {
			id = id[:12]
		}
		display := id
		if nameValue, _ := body["name"].(string); nameValue != "" {
			display = nameValue
		}
		return fmt.Sprintf("✅ Container %s created and started (ID: %s)", display, id)
	case "list_images":
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/images", nil, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var images []model.ImageInfo
		if err := json.Unmarshal(raw, &images); err != nil {
			return "Error: " + err.Error()
		}
		var b strings.Builder
		b.WriteString("| ID | 名称 | 大小 |\n| --- | --- | --- |\n")
		for _, img := range images {
			tag := img.Name
			if img.Tag != "" {
				tag += ":" + img.Tag
			}
			if tag == "" {
				tag = "<none>"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", img.ID, tag, img.Size))
		}
		return b.String()
	case "pull_image":
		if arg1 == "" {
			return "Missing image name"
		}
		_, err := s.callNode(ctx, address, http.MethodPost, "/api/images/pull", nil, map[string]string{"name": arg1})
		if err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("✅ Image %s pulled successfully.", arg1)
	case "delete_image":
		if arg1 == "" {
			return "Missing image ID"
		}
		_, err := s.callNode(ctx, address, http.MethodPost, "/api/images/remove", nil, map[string]string{"id": arg1})
		if err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("✅ Image %s deleted.", arg1)
	case "list_networks":
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/networks", nil, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var networks []model.NetworkInfo
		if err := json.Unmarshal(raw, &networks); err != nil {
			return "Error: " + err.Error()
		}
		var b strings.Builder
		b.WriteString("| ID | 名称 | 驱动 | 范围 |\n| --- | --- | --- | --- |\n")
		for _, n := range networks {
			id := n.ID
			if len(id) > 12 {
				id = id[:12]
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", id, n.Name, n.Driver, n.Scope))
		}
		return b.String()
	case "create_network":
		if arg1 == "" {
			return "Missing network name"
		}
		driver := "bridge"
		if len(args) > 1 && args[1] != "" {
			driver = args[1]
		}
		_, err := s.callNode(ctx, address, http.MethodPost, "/api/networks/create", nil, map[string]string{"name": arg1, "driver": driver})
		if err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("✅ Network %s created.", arg1)
	case "delete_network":
		if arg1 == "" {
			return "Missing network ID"
		}
		_, err := s.callNode(ctx, address, http.MethodPost, "/api/networks/remove", nil, map[string]string{"id": arg1})
		if err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("✅ Network %s deleted.", arg1)
	case "inspect_network":
		if arg1 == "" {
			return "Missing network ID"
		}
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/networks/inspect", url.Values{"id": {arg1}}, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var info model.NetworkDetail
		if err := json.Unmarshal(raw, &info); err != nil {
			return "Error: " + err.Error()
		}
		id := info.ID
		if len(id) > 12 {
			id = id[:12]
		}
		return fmt.Sprintf("**网络**: %s\n\n| 属性 | 值 |\n| --- | --- |\n| ID | %s |\n| 驱动 | %s |\n| 范围 | %s |\n| 容器数 | %d |\n", info.Name, id, info.Driver, info.Scope, info.ContainerCount)
	case "list_volumes":
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/volumes", nil, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var volumes []model.VolumeInfo
		if err := json.Unmarshal(raw, &volumes); err != nil {
			return "Error: " + err.Error()
		}
		var b strings.Builder
		b.WriteString("| 名称 | 驱动 |\n| --- | --- |\n")
		for _, v := range volumes {
			b.WriteString(fmt.Sprintf("| %s | %s |\n", v.Name, v.Driver))
		}
		return b.String()
	case "create_volume":
		if arg1 == "" {
			return "Missing volume name"
		}
		_, err := s.callNode(ctx, address, http.MethodPost, "/api/volumes/create", nil, map[string]string{"name": arg1})
		if err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("✅ Volume %s created.", arg1)
	case "delete_volume":
		if arg1 == "" {
			return "Missing volume name"
		}
		_, err := s.callNode(ctx, address, http.MethodPost, "/api/volumes/remove", nil, map[string]string{"name": arg1})
		if err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("✅ Volume %s deleted.", arg1)
	case "list_compose_projects":
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/compose/list", nil, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		return formatComposeProjects(raw)
	case "compose_status":
		if arg1 == "" {
			return "Missing project name"
		}
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/compose/status", url.Values{"project": {arg1}}, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var project model.ComposeProject
		if err := json.Unmarshal(raw, &project); err != nil {
			return "Error: " + err.Error()
		}
		return formatOneCompose(project)
	case "compose_up", "compose_down", "compose_restart":
		if arg1 == "" {
			return "Missing project name"
		}
		action := strings.TrimPrefix(name, "compose_")
		raw, err := s.callNode(ctx, address, http.MethodPost, "/api/compose/action", nil, map[string]string{"project": arg1, "action": action})
		if err != nil {
			return "Error: " + err.Error()
		}
		logs, failed := parseSSE(string(raw))
		if failed != "" {
			return fmt.Sprintf("Error: %s\n%s", failed, logs)
		}
		return fmt.Sprintf("✅ Compose project %s %s.\n```\n%s\n```", arg1, action, logs)
	case "system_status":
		raw, err := s.callNode(ctx, address, http.MethodGet, "/api/system/stats", nil, nil)
		if err != nil {
			return "Error: " + err.Error()
		}
		var stats model.SystemStats
		if err := json.Unmarshal(raw, &stats); err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("| 指标 | 使用率 |\n| --- | --- |\n| CPU | %.1f%% |\n| 内存 | %.1f%% |\n| 磁盘 | %.1f%% |", stats.CPU, stats.Memory, stats.Disk)
	case "prune_containers", "prune_images":
		return "这台远程节点暂不支持在智能体里清理容器或镜像。"
	default:
		return "Unknown tool: " + name
	}
}

func (s *AgentService) callNode(ctx context.Context, address, method, path string, query url.Values, body any) ([]byte, error) {
	if len(s.jwtSecret) == 0 {
		return nil, fmt.Errorf("主节点缺少 JWT 密钥，无法调用其他节点")
	}
	token, err := middleware.GenerateToken("rabbit-agent", false, s.jwtSecret)
	if err != nil {
		return nil, err
	}
	endpoint := "http://" + address + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	timeout := 30 * time.Second
	switch path {
	case "/api/images/pull", "/api/containers/run", "/api/compose/action":
		timeout = 3 * time.Minute
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("节点不可达: %s", err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		var wrap struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &wrap) == nil && wrap.Error != "" {
			msg = wrap.Error
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return raw, nil
}

func remoteRunBody(args []string) (map[string]any, string) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return nil, "Missing image name"
	}
	body := map[string]any{"image": args[0]}
	if len(args) > 1 && args[1] != "" {
		body["name"] = args[1]
	}
	var ports, envs, volumes []map[string]string
	for _, opt := range args[2:] {
		parts := splitToolOption(opt)
		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}
		switch key {
		case "-p", "--publish":
			if host, containerPort := splitHostContainer(val); host != "" && containerPort != "" {
				ports = append(ports, map[string]string{"host": host, "container": containerPort})
			}
		case "-e", "--env":
			if k, v, ok := strings.Cut(val, "="); ok && strings.TrimSpace(k) != "" {
				envs = append(envs, map[string]string{"key": strings.TrimSpace(k), "value": v})
			}
		case "-v", "--volume":
			if host, containerPath := splitHostContainer(val); host != "" && containerPath != "" {
				volumes = append(volumes, map[string]string{"host": host, "container": containerPath})
			}
		case "--restart":
			body["restart"] = val
		case "--network", "--net":
			body["network"] = val
		default:
			if host, containerPort := splitHostContainer(opt); host != "" && containerPort != "" && !strings.Contains(opt, "=") {
				ports = append(ports, map[string]string{"host": host, "container": containerPort})
			}
		}
	}
	if len(ports) > 0 {
		body["ports"] = ports
	}
	if len(envs) > 0 {
		body["env"] = envs
	}
	if len(volumes) > 0 {
		body["volumes"] = volumes
	}
	return body, ""
}

func splitHostContainer(value string) (string, string) {
	host, containerPart, ok := strings.Cut(value, ":")
	if !ok {
		return "", ""
	}
	return strings.TrimSpace(host), strings.TrimSpace(containerPart)
}

func formatComposeProjects(raw []byte) string {
	var projects []model.ComposeProject
	if err := json.Unmarshal(raw, &projects); err != nil {
		return "Error: " + err.Error()
	}
	var b strings.Builder
	b.WriteString("| 项目名 | 状态 |\n| --- | --- |\n")
	for _, p := range projects {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", p.Name, p.Status))
	}
	return b.String()
}

func formatOneCompose(project model.ComposeProject) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**项目 %s 状态:** %s\n", project.Name, project.Status))
	if len(project.Containers) == 0 {
		return b.String()
	}
	b.WriteString("\n| 名称 | 状态 |\n| --- | --- |\n")
	for _, c := range project.Containers {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", c.Name, c.Status))
	}
	return b.String()
}

func sseText(raw string) string {
	logs, _ := parseSSE(raw)
	if logs == "" {
		return strings.TrimSpace(raw)
	}
	return logs
}

func parseSSE(raw string) (string, string) {
	var lines []string
	failed := ""
	event := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if event == "error" || data == "failed" {
				if data != "" && data != "failed" {
					failed = data
				} else if failed == "" {
					failed = "failed"
				}
				continue
			}
			if event == "done" || data == "success" || data == "" {
				continue
			}
			lines = append(lines, data)
		}
	}
	return strings.Join(lines, "\n"), failed
}

func parseToolCommand(command string) (string, []string, string, bool) {
	command = strings.TrimSpace(command)
	idx := strings.Index(command, "(")
	if idx < 1 || !strings.HasSuffix(command, ")") {
		return "", nil, "", false
	}
	name := strings.TrimSpace(command[:idx])
	var args []string
	node := ""
	for _, part := range splitToolArgs(command[idx+1 : len(command)-1]) {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if key, val, ok := strings.Cut(trimmed, "="); ok {
			key = strings.TrimSpace(key)
			val = strings.Trim(strings.TrimSpace(val), "\"'")
			if key == "node" || key == "节点" {
				node = val
				continue
			}
			trimmed = val
		} else {
			trimmed = strings.Trim(trimmed, "\"'")
		}
		if trimmed != "" {
			args = append(args, trimmed)
		}
	}
	return name, args, node, name != ""
}

func splitToolArgs(input string) []string {
	var parts []string
	var b strings.Builder
	var quote byte
	for i := 0; i < len(input); i++ {
		c := input[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == ',' {
			parts = append(parts, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	parts = append(parts, b.String())
	return parts
}

func sortedNodes(list []*model.NodeInfo) []*model.NodeInfo {
	out := append([]*model.NodeInfo(nil), list...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mode != out[j].Mode {
			return out[i].Mode == model.ModeMaster
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func nodeModeText(mode string) string {
	if mode == model.ModeWorker {
		return "工作节点"
	}
	return "主节点"
}

func nodeStatusText(status string) string {
	switch status {
	case model.NodeStatusOnline:
		return "在线"
	case model.NodeStatusOffline:
		return "离线"
	default:
		if status == "" {
			return "未知"
		}
		return status
	}
}
