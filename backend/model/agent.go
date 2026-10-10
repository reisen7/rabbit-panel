package model

// AgentConfig 智能体配置
type AgentConfig struct {
	APIURL         string `json:"api_url"`
	APIKey         string `json:"api_key"`
	APIFormat      string `json:"api_format"`
	Model          string `json:"model"`
	Enabled        bool   `json:"enabled"`
	Thinking       bool   `json:"thinking"`
	ThinkingEffort string `json:"thinking_effort"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest OpenAI 格式的聊天请求
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatCompletionChunk 流式响应片段
type ChatCompletionChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// AgentChatRequest AI 智能体聊天请求
type AgentChatRequest struct {
	Message string        `json:"message"`
	History []ChatMessage `json:"history"`
	NodeID  string        `json:"node_id"`
}

// ChatHistoryMessage 聊天历史记录
type ChatHistoryMessage struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// AgentChatResponse AI 聊天响应
type AgentChatResponse struct {
	Reply   string `json:"reply"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
