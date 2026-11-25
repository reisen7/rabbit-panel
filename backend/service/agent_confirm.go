package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ChatEvent 是智能体对话流向客户端推送的一条结构化事件。
// Type 取值：
//   - "chunk"   文本片段（Text）
//   - "confirm" 危险操作待确认（ID + Command + Summary）
//   - "error"   错误（Text）
type ChatEvent struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	ID      string `json:"id,omitempty"`
	Command string `json:"command,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// confirmTimeout 是面板端确认的最长等待时间，超时自动拒绝。
const confirmTimeout = 5 * time.Minute

// pendingConfirm 是一条等待用户确认的危险操作。
type pendingConfirm struct {
	id      string
	command string
	answer  chan bool
	created time.Time
}

// confirmStore 存放进行中的待确认操作。键为随机 ID。
type confirmStore struct {
	mu sync.Mutex
	m  map[string]*pendingConfirm
}

func newConfirmStore() *confirmStore {
	return &confirmStore{m: make(map[string]*pendingConfirm)}
}

// add 注册一条待确认操作，返回待确认对象（ID 在 id 字段）。
// 调用方持对象直接 wait，避免与 resolve 的清理产生竞态。
func (s *confirmStore) add(command string) (*pendingConfirm, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	p := &pendingConfirm{
		id:      hex.EncodeToString(b),
		command: command,
		answer:  make(chan bool, 1),
		created: time.Now(),
	}
	s.mu.Lock()
	s.m[p.id] = p
	s.mu.Unlock()
	return p, nil
}

// resolve 响应一次确认。操作不存在（或已超时清理）时返回 false。
// answer 通道带缓冲，发送永不阻塞；删表先于发送保证一次性。
func (s *confirmStore) resolve(id string, approve bool) bool {
	s.mu.Lock()
	p, ok := s.m[id]
	if ok {
		delete(s.m, id)
	}
	s.mu.Unlock()
	if !ok {
		return false
	}
	p.answer <- approve
	return true
}

// wait 阻塞等待用户答复；超时或流断开（ctx 取消）时返回 false。
// 无论结果如何都会清理该条记录（与 resolve 的删除互为幂等）。
func (s *confirmStore) wait(ctx context.Context, p *pendingConfirm, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var approve bool
	select {
	case approve = <-p.answer:
	case <-ctx.Done():
	case <-timer.C:
	}
	s.mu.Lock()
	delete(s.m, p.id)
	s.mu.Unlock()
	return approve
}

// needsConfirm 返回需要人工确认的危险操作集合：
// 停止/重启/删除/清理类与批量变更类，覆盖系统提示词里"关键操作"的定义。
func needsConfirm(toolName string) bool {
	switch toolName {
	case "stop_container",
		"restart_container",
		"delete_container",
		"delete_image",
		"delete_network",
		"delete_volume",
		"compose_down",
		"compose_restart",
		"prune_containers",
		"prune_images":
		return true
	}
	return false
}

// confirmSummary 生成给用户看的一句话操作描述。
func confirmSummary(toolName string, args []string) string {
	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	switch toolName {
	case "stop_container":
		return fmt.Sprintf("停止容器 %s", target)
	case "restart_container":
		return fmt.Sprintf("重启容器 %s", target)
	case "delete_container":
		return fmt.Sprintf("强制删除容器 %s", target)
	case "delete_image":
		return fmt.Sprintf("强制删除镜像 %s", target)
	case "delete_network":
		return fmt.Sprintf("删除网络 %s", target)
	case "delete_volume":
		return fmt.Sprintf("删除存储卷 %s", target)
	case "compose_down":
		return fmt.Sprintf("停止并移除 Compose 项目 %s", target)
	case "compose_restart":
		return fmt.Sprintf("重启 Compose 项目 %s", target)
	case "prune_containers":
		return "清理所有已停止的容器"
	case "prune_images":
		return "清理所有未使用的镜像"
	}
	return "执行操作 " + strings.TrimSpace(toolName+" "+target)
}

// ResolveConfirm 响应一条待确认操作。供 HTTP 端点调用。
func (s *AgentService) ResolveConfirm(id string, approve bool) bool {
	return s.confirms.resolve(id, approve)
}
