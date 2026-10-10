package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"rabbit-panel/agent"
	"rabbit-panel/model"
	"rabbit-panel/repository"
)

// AgentService AI 智能体服务
type AgentService struct {
	sqliteRepo *repository.SQLiteRepository
	dockerRepo repository.IDockerRepository
	registry   *RegistryService
	nodes      *NodeService
	jwtSecret  []byte

	config      AgentConfig
	configMutex sync.RWMutex
	configPath  string
}

// AgentConfig AI 配置。
// APIFormat：anthropic、openai_chat、openai_responses。
// Thinking 打开后按 ThinkingEffort（low、medium、high、xhigh）向对应接口发送思考参数。
type AgentConfig struct {
	APIURL         string `json:"api_url"`
	APIKey         string `json:"api_key"`
	APIFormat      string `json:"api_format"`
	Model          string `json:"model"`
	Enabled        bool   `json:"enabled"`
	Thinking       bool   `json:"thinking"`
	ThinkingEffort string `json:"thinking_effort"`
}

// NewAgentService 创建 AI 服务
func NewAgentService(sr *repository.SQLiteRepository, dr repository.IDockerRepository) *AgentService {
	service := &AgentService{
		sqliteRepo: sr,
		dockerRepo: dr,
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

// UseNodes 让智能体识别并操作主节点登记的其他节点。
func (s *AgentService) UseNodes(nodes *NodeService, jwtSecret []byte) {
	s.nodes = nodes
	s.jwtSecret = jwtSecret
}

// GetConfig 获取配置
func (s *AgentService) GetConfig() AgentConfig {
	s.configMutex.RLock()
	cfg := s.config
	s.configMutex.RUnlock()
	return normalizeAgentConfig(cfg)
}

// SaveConfig 保存配置
func (s *AgentService) SaveConfig(cfg AgentConfig) error {
	s.configMutex.Lock()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.Contains(cfg.APIKey, "****") {
		cfg.APIKey = s.config.APIKey
	}
	s.config = normalizeAgentConfig(cfg)
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

// Complete 同步跑一轮对话，供微信等通道收集完整回复。
func (s *AgentService) Complete(ctx context.Context, message string, history []model.ChatMessage) (string, error) {
	var reply strings.Builder
	err := s.StreamChat(ctx, model.AgentChatRequest{Message: message, History: history}, func(chunk string) error {
		reply.WriteString(chunk)
		return nil
	})
	return strings.TrimSpace(reply.String()), err
}

func (s *AgentService) StreamChat(ctx context.Context, req model.AgentChatRequest, onChunk func(string) error) error {
	cfg := s.GetConfig()
	if !cfg.Enabled {
		return fmt.Errorf("agent is disabled")
	}
	if strings.TrimSpace(cfg.APIURL) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("agent config is incomplete")
	}

	if strings.TrimSpace(req.NodeID) != "" {
		ctx = context.WithValue(ctx, agentNodeKey{}, strings.TrimSpace(req.NodeID))
	}

	messages := make([]model.ChatMessage, 0, len(req.History)+2)
	messages = append(messages, model.ChatMessage{Role: "system", Content: agent.SystemPrompt + s.nodePrompt(req.NodeID)})
	messages = append(messages, req.History...)
	messages = append(messages, model.ChatMessage{Role: "user", Content: req.Message})

	for step := 0; step < 3; step++ {
		fallback := ""
		if step == 0 {
			fallback = s.withMentionedNode(fallbackToolCommand(req.Message), req.Message)
		}
		var withheld strings.Builder
		sink := onChunk
		if fallback != "" {
			sink = func(chunk string) error {
				withheld.WriteString(chunk)
				return nil
			}
		}
		responseText, err := s.callLLMStream(ctx, cfg, messages, sink)
		if err != nil {
			return err
		}
		messages = append(messages, model.ChatMessage{Role: "assistant", Content: responseText})

		toolResults, err := s.executeToolCalls(ctx, responseText, sink)
		if err != nil {
			return err
		}
		if len(toolResults) == 0 && fallback != "" {
			toolResults, err = s.executeToolCalls(ctx, "[[TOOL: "+fallback+"]]", onChunk)
			if err != nil {
				return err
			}
		} else if fallback != "" && withheld.Len() > 0 {
			if err := onChunk(withheld.String()); err != nil {
				return err
			}
		}
		if len(toolResults) == 0 {
			return nil
		}
		messages = append(messages, model.ChatMessage{
			Role: "user",
			Content: fmt.Sprintf("[SYSTEM] 工具已在对应节点上执行完成，下面是实时结果。请只根据结果用中文总结，并说明是哪一台节点。不要说没有连接 Docker，不要让用户自己执行命令。\n%s",
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
			s.config = normalizeAgentConfig(AgentConfig{})
			s.config.Enabled = false
			return nil
		}
		return err
	}
	defer file.Close()

	var cfg AgentConfig
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return err
	}
	s.config = normalizeAgentConfig(cfg)
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
