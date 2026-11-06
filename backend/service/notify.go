package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const showdocPushHost = "https://push.showdoc.com.cn/server/api/push/"

var showdocTokenPattern = regexp.MustCompile(`^[A-Za-z0-9]{8,128}$`)

// NotifyConfig 是 Showdoc 异常通知的绑定设置。Token 只存在服务端。
type NotifyConfig struct {
	Enabled         bool     `json:"enabled"`
	Token           string   `json:"token,omitempty"`
	TokenMask       string   `json:"token_mask"`
	HasToken        bool     `json:"has_token"`
	AllContainers   bool     `json:"all_containers"`
	Containers      []string `json:"containers"`
	ExitNonZero     bool     `json:"exit_nonzero"`
	Unhealthy       bool     `json:"unhealthy"`
	RestartStorm    bool     `json:"restart_storm"`
	Recovery        bool     `json:"recovery"`
	CooldownMinutes int      `json:"cooldown_minutes"`
}

// NotifyService 保存通知绑定，并负责向 Showdoc 推送服务发测试消息。
type NotifyService struct {
	mu     sync.RWMutex
	path   string
	config NotifyConfig
	client *http.Client
}

// NewNotifyService 从 data/notify.json 读取绑定。文件不存在时使用默认规则。
func NewNotifyService() *NotifyService {
	service := &NotifyService{
		path:   "./data/notify.json",
		config: defaultNotifyConfig(),
		client: &http.Client{Timeout: 15 * time.Second},
	}
	_ = service.load()
	return service
}

func defaultNotifyConfig() NotifyConfig {
	return NotifyConfig{
		AllContainers:   true,
		ExitNonZero:     true,
		Unhealthy:       true,
		RestartStorm:    true,
		Recovery:        true,
		CooldownMinutes: 10,
		Containers:      []string{},
	}
}

// Settings 返回给页面的配置。Token 原文不会带出去。
func (s *NotifyService) Settings() NotifyConfig {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()
	return publicNotifyConfig(cfg)
}

// Save 写入绑定。Token 留空或仍是掩码时，沿用已经保存的值。
func (s *NotifyService) Save(input NotifyConfig) (NotifyConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.config
	token, keep, err := normalizeShowdocToken(input.Token)
	if err != nil {
		return NotifyConfig{}, err
	}
	if !keep {
		next.Token = token
	}
	names, err := normalizeContainerNames(input.Containers)
	if err != nil {
		return NotifyConfig{}, err
	}
	next.Enabled = input.Enabled
	next.AllContainers = input.AllContainers
	next.Containers = names
	next.ExitNonZero = input.ExitNonZero
	next.Unhealthy = input.Unhealthy
	next.RestartStorm = input.RestartStorm
	next.Recovery = input.Recovery
	next.CooldownMinutes = input.CooldownMinutes
	if next.CooldownMinutes < 1 {
		next.CooldownMinutes = 1
	}
	if next.CooldownMinutes > 1440 {
		next.CooldownMinutes = 1440
	}
	s.config = next
	if err := s.persistLocked(); err != nil {
		return NotifyConfig{}, err
	}
	return publicNotifyConfig(next), nil
}

// Test 向固定的 Showdoc 推送地址发一条测试消息。传入的 Token 只用于这一次，不会写入文件。
func (s *NotifyService) Test(ctx context.Context, token string) error {
	normalized, keep, err := normalizeShowdocToken(token)
	if err != nil {
		return err
	}
	if keep {
		s.mu.RLock()
		normalized = s.config.Token
		s.mu.RUnlock()
	}
	if normalized == "" {
		return fmt.Errorf("请先填写 Showdoc Token")
	}
	return s.push(ctx, normalized, "Rabbit Panel 测试", "这是一条测试消息。收到它，说明通知绑定已经接通。")
}

func (s *NotifyService) push(ctx context.Context, token, title, content string) error {
	payload, err := json.Marshal(map[string]string{"title": title, "content": content})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, showdocPushHost+token, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送失败，请检查面板能否访问 push.showdoc.com.cn")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Showdoc 推送返回 %d", resp.StatusCode)
	}
	var result struct {
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.ErrorCode != 0 {
		message := strings.TrimSpace(result.ErrorMessage)
		if message == "" {
			message = "推送被拒绝"
		}
		return fmt.Errorf("Showdoc：%s", message)
	}
	return nil
}

func (s *NotifyService) load() error {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	var cfg NotifyConfig
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return err
	}
	if cfg.Containers == nil {
		cfg.Containers = []string{}
	}
	if cfg.CooldownMinutes <= 0 {
		cfg.CooldownMinutes = 10
	}
	s.config = cfg
	return nil
}

func (s *NotifyService) persistLocked() error {
	if err := os.MkdirAll("./data", 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(payload, '\n'), 0600)
}

func publicNotifyConfig(cfg NotifyConfig) NotifyConfig {
	cfg.HasToken = cfg.Token != ""
	cfg.TokenMask = maskToken(cfg.Token)
	cfg.Token = ""
	if cfg.Containers == nil {
		cfg.Containers = []string{}
	}
	return cfg
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// normalizeShowdocToken 取出 push/ 后面的一段。空字符串和掩码表示保留原 Token。
func normalizeShowdocToken(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "****") {
		return "", true, nil
	}
	if i := strings.LastIndex(strings.ToLower(raw), "/push/"); i >= 0 {
		raw = raw[i+len("/push/"):]
	}
	raw = strings.Trim(raw, "/")
	raw = strings.TrimSpace(raw)
	if !showdocTokenPattern.MatchString(raw) {
		return "", false, fmt.Errorf("Token 只能是数字和字母，请只填写 push/ 后面的那一串")
	}
	return raw, false, nil
}

func normalizeContainerNames(names []string) ([]string, error) {
	if len(names) > 100 {
		return nil, fmt.Errorf("选择的容器太多")
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if len(name) > 128 || strings.ContainsAny(name, " /\t\r\n") {
			return nil, fmt.Errorf("容器名无效：%s", name)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}
