package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"rabbit-panel/model"
)

const (
	formatAnthropic       = "anthropic"
	formatOpenAIChat      = "openai_chat"
	formatOpenAIResponses = "openai_responses"
	anthropicVersion      = "2023-06-01"
)

type llmRequest struct {
	URL    string
	Header http.Header
	Body   []byte
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func normalizeAgentConfig(cfg AgentConfig) AgentConfig {
	cfg.APIURL = strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.openai.com/v1"
	}
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = "gpt-3.5-turbo"
	}
	switch cfg.APIFormat {
	case formatAnthropic, formatOpenAIChat, formatOpenAIResponses:
	default:
		cfg.APIFormat = formatOpenAIChat
	}
	switch cfg.ThinkingEffort {
	case "low", "medium", "high", "xhigh":
	default:
		cfg.ThinkingEffort = "medium"
	}
	return cfg
}

func buildLLMRequest(cfg AgentConfig, messages []model.ChatMessage) (llmRequest, error) {
	cfg = normalizeAgentConfig(cfg)
	switch cfg.APIFormat {
	case formatAnthropic:
		return buildAnthropicRequest(cfg, messages)
	case formatOpenAIResponses:
		return buildResponsesRequest(cfg, messages)
	default:
		return buildOpenAIChatRequest(cfg, messages)
	}
}

func buildOpenAIChatRequest(cfg AgentConfig, messages []model.ChatMessage) (llmRequest, error) {
	body := map[string]any{
		"model":    cfg.Model,
		"messages": messages,
		"stream":   true,
	}
	if cfg.Thinking {
		body["reasoning_effort"] = cfg.ThinkingEffort
	}
	return encodeLLM(cfg.APIURL+"/chat/completions", openAIHeaders(cfg.APIKey), body)
}

func buildResponsesRequest(cfg AgentConfig, messages []model.ChatMessage) (llmRequest, error) {
	body := map[string]any{
		"model":  cfg.Model,
		"input":  messages,
		"stream": true,
	}
	if cfg.Thinking {
		body["reasoning"] = map[string]string{"effort": cfg.ThinkingEffort}
	}
	return encodeLLM(cfg.APIURL+"/responses", openAIHeaders(cfg.APIKey), body)
}

func buildAnthropicRequest(cfg AgentConfig, messages []model.ChatMessage) (llmRequest, error) {
	system, conv, err := anthropicConversation(messages)
	if err != nil {
		return llmRequest{}, err
	}
	maxTokens := 8192
	body := map[string]any{
		"model":      cfg.Model,
		"messages":   conv,
		"max_tokens": maxTokens,
		"stream":     true,
	}
	if system != "" {
		body["system"] = system
	}
	if cfg.Thinking {
		budget := thinkingBudget(cfg.ThinkingEffort)
		body["max_tokens"] = budget + 4096
		body["thinking"] = map[string]any{
			"type":          "enabled",
			"budget_tokens": budget,
		}
	}
	return encodeLLM(anthropicMessagesURL(cfg.APIURL), anthropicHeaders(cfg.APIKey), body)
}

func encodeLLM(url string, header http.Header, body any) (llmRequest, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return llmRequest{}, err
	}
	return llmRequest{URL: url, Header: header, Body: raw}, nil
}

func openAIHeaders(apiKey string) http.Header {
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+apiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "text/event-stream")
	return header
}

func anthropicHeaders(apiKey string) http.Header {
	header := make(http.Header)
	header.Set("x-api-key", apiKey)
	header.Set("anthropic-version", anthropicVersion)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "text/event-stream")
	return header
}

func anthropicMessagesURL(base string) string {
	if strings.HasSuffix(base, "/v1") {
		return base + "/messages"
	}
	return base + "/v1/messages"
}

func anthropicModelsURL(base string) string {
	if strings.HasSuffix(base, "/v1") {
		return base + "/models?limit=100"
	}
	return base + "/v1/models?limit=100"
}

func thinkingBudget(effort string) int {
	switch effort {
	case "low":
		return 1024
	case "high":
		return 16000
	case "xhigh":
		return 64000
	default:
		return 4096
	}
}

func anthropicConversation(messages []model.ChatMessage) (string, []anthropicMessage, error) {
	var systems []string
	var conv []anthropicMessage
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			systems = append(systems, msg.Content)
		case "assistant":
			conv = appendAnthropic(conv, "assistant", msg.Content)
		default:
			conv = appendAnthropic(conv, "user", msg.Content)
		}
	}
	if len(conv) == 0 {
		return "", nil, fmt.Errorf("agent messages are empty")
	}
	if conv[0].Role != "user" {
		conv = append([]anthropicMessage{{Role: "user", Content: "继续"}}, conv...)
	}
	return strings.Join(systems, "\n\n"), conv, nil
}

func appendAnthropic(conv []anthropicMessage, role, content string) []anthropicMessage {
	if len(conv) > 0 && conv[len(conv)-1].Role == role {
		conv[len(conv)-1].Content += "\n\n" + content
		return conv
	}
	return append(conv, anthropicMessage{Role: role, Content: content})
}

func readLLMStream(format string, body io.Reader, onChunk func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var full strings.Builder
	var dataLines []string
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if payload == "" || payload == "[DONE]" {
			return nil
		}
		text, err := extractStreamText(format, payload)
		if err != nil {
			return err
		}
		if text == "" {
			return nil
		}
		full.WriteString(text)
		if onChunk != nil {
			return onChunk(text)
		}
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return full.String(), err
			}
			continue
		}
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "data:"); ok {
			dataLines = append(dataLines, strings.TrimSpace(rest))
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") {
			dataLines = append(dataLines, trimmed)
			if err := flush(); err != nil {
				return full.String(), err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), err
	}
	if err := flush(); err != nil {
		return full.String(), err
	}
	return full.String(), nil
}

func extractStreamText(format, payload string) (string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return "", fmt.Errorf("agent stream parse failed: %w", err)
	}
	if err := streamError(raw); err != nil {
		return "", err
	}
	switch format {
	case formatAnthropic:
		return anthropicDelta(raw)
	case formatOpenAIResponses:
		return responsesDelta(raw)
	default:
		return openAIChatDelta(raw)
	}
}

func streamError(raw map[string]json.RawMessage) error {
	if msg := errorMessage(raw["error"]); msg != "" {
		return fmt.Errorf("agent api stream error: %s", msg)
	}
	switch jsonString(raw["type"]) {
	case "error", "response.failed", "response.error":
		if msg := nestedStreamError(raw); msg != "" {
			return fmt.Errorf("agent api stream error: %s", msg)
		}
		return fmt.Errorf("agent api stream error")
	default:
		return nil
	}
}

func nestedStreamError(raw map[string]json.RawMessage) string {
	if msg := errorMessage(raw["error"]); msg != "" {
		return msg
	}
	if msg := jsonString(raw["message"]); msg != "" {
		return msg
	}
	resp, ok := raw["response"]
	if !ok {
		return ""
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(resp, &inner) != nil {
		return ""
	}
	return errorMessage(inner["error"])
}

func errorMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	msg, _ := obj["message"].(string)
	return strings.TrimSpace(msg)
}

func jsonString(raw json.RawMessage) string {
	var text string
	if len(raw) == 0 || json.Unmarshal(raw, &text) != nil {
		return ""
	}
	return text
}

func openAIChatDelta(raw map[string]json.RawMessage) (string, error) {
	choices, ok := raw["choices"]
	if !ok {
		return "", nil
	}
	var arr []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	}
	if json.Unmarshal(choices, &arr) != nil || len(arr) == 0 {
		return "", nil
	}
	return arr[0].Delta.Content, nil
}

func responsesDelta(raw map[string]json.RawMessage) (string, error) {
	if jsonString(raw["type"]) != "response.output_text.delta" {
		return "", nil
	}
	delta, ok := raw["delta"]
	if !ok {
		return "", nil
	}
	var text string
	if json.Unmarshal(delta, &text) == nil {
		return text, nil
	}
	var obj struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(delta, &obj) != nil {
		return "", nil
	}
	return obj.Text, nil
}

func anthropicDelta(raw map[string]json.RawMessage) (string, error) {
	if jsonString(raw["type"]) != "content_block_delta" {
		return "", nil
	}
	var delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw["delta"], &delta) != nil || delta.Type != "text_delta" {
		return "", nil
	}
	return delta.Text, nil
}

func (s *AgentService) callLLMStream(ctx context.Context, cfg AgentConfig, messages []model.ChatMessage, onChunk func(string) error) (string, error) {
	built, err := buildLLMRequest(cfg, messages)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, built.URL, bytes.NewReader(built.Body))
	if err != nil {
		return "", err
	}
	for key, values := range built.Header {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	timeout := 90 * time.Second
	if cfg.Thinking {
		timeout = 180 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("agent api request failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return readLLMStream(cfg.APIFormat, resp.Body, onChunk)
}

// TestConnection 用当前填写的地址、格式、Key 和模型发一条 ping。
// 思考参数不发送，避免测试把高额度思考预算打出去。空 Key 或脱敏 Key 使用已保存的 Key。
func (s *AgentService) TestConnection(ctx context.Context, cfg AgentConfig) (string, error) {
	cfg.APIURL = strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	cfg.APIFormat = strings.TrimSpace(cfg.APIFormat)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.APIKey == "" || strings.Contains(cfg.APIKey, "****") {
		s.configMutex.RLock()
		cfg.APIKey = strings.TrimSpace(s.config.APIKey)
		s.configMutex.RUnlock()
	}
	if cfg.APIURL == "" || cfg.APIFormat == "" || cfg.APIKey == "" || cfg.Model == "" {
		return "", fmt.Errorf("请先填写 API URL、格式、API Key 和模型")
	}
	switch cfg.APIFormat {
	case formatAnthropic, formatOpenAIChat, formatOpenAIResponses:
	default:
		return "", fmt.Errorf("不支持的接口格式")
	}
	cfg.Thinking = false
	cfg = normalizeAgentConfig(cfg)

	reqCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	reply, err := s.callLLMStream(reqCtx, cfg, []model.ChatMessage{{
		Role:    "user",
		Content: "ping",
	}}, func(string) error { return nil })
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("连接超时")
		}
		return "", fmt.Errorf("连接失败: %s", clipRunes(strings.TrimSpace(err.Error()), 300))
	}
	return clipRunes(strings.TrimSpace(reply), 200), nil
}

// ListModels 按接口格式拉取模型 id。空 Key 或脱敏 Key 使用已保存的 Key。
func (s *AgentService) ListModels(ctx context.Context, apiURL, apiFormat, apiKey string) ([]string, error) {
	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	apiFormat = strings.TrimSpace(apiFormat)
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || strings.Contains(apiKey, "****") {
		s.configMutex.RLock()
		apiKey = strings.TrimSpace(s.config.APIKey)
		s.configMutex.RUnlock()
	}
	if apiURL == "" || apiFormat == "" || apiKey == "" {
		return nil, fmt.Errorf("请先填写 API URL、格式和 API Key")
	}
	switch apiFormat {
	case formatAnthropic, formatOpenAIChat, formatOpenAIResponses:
	default:
		return nil, fmt.Errorf("不支持的接口格式")
	}

	endpoint := apiURL + "/models"
	header := openAIHeaders(apiKey)
	header.Set("Accept", "application/json")
	if apiFormat == formatAnthropic {
		endpoint = anthropicModelsURL(apiURL)
		header = anthropicHeaders(apiKey)
		header.Set("Accept", "application/json")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("获取模型失败: %w", err)
	}
	req.Header = header
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取模型失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("获取模型失败: %s %s", resp.Status, clipRunes(strings.TrimSpace(string(raw)), 300))
	}
	ids, err := parseModelIDs(raw)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

func parseModelIDs(raw []byte) ([]string, error) {
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("无法解析模型列表")
	}
	seen := map[string]struct{}{}
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, item := range payload.Data {
		add(item.ID)
	}
	for _, item := range payload.Models {
		add(item.ID)
	}
	sort.Strings(ids)
	return ids, nil
}

func clipRunes(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n])
}
