package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"

	"rabbit-panel/model"
	"rabbit-panel/repository"
)

func TestBuildLLMRequestFormats(t *testing.T) {
	messages := []model.ChatMessage{
		{Role: "system", Content: "规则"},
		{Role: "user", Content: "你好"},
	}

	chat, err := buildLLMRequest(AgentConfig{
		APIURL:    "https://api.openai.com/v1/",
		APIKey:    "sk-chat",
		APIFormat: formatOpenAIChat,
		Model:     "gpt-4o",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	if chat.URL != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("chat url %s", chat.URL)
	}
	if chat.Header.Get("Authorization") != "Bearer sk-chat" {
		t.Fatalf("chat auth %s", chat.Header.Get("Authorization"))
	}
	assertAbsent(t, chat.Body, "reasoning_effort")

	thinking, err := buildLLMRequest(AgentConfig{
		APIURL:         "https://api.openai.com/v1",
		APIKey:         "sk-chat",
		APIFormat:      formatOpenAIChat,
		Model:          "gpt-4o",
		Thinking:       true,
		ThinkingEffort: "high",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	var chatBody map[string]any
	if err := json.Unmarshal(thinking.Body, &chatBody); err != nil {
		t.Fatal(err)
	}
	if chatBody["reasoning_effort"] != "high" || chatBody["stream"] != true {
		t.Fatalf("chat thinking body %#v", chatBody)
	}

	responses, err := buildLLMRequest(AgentConfig{
		APIURL:         "https://api.openai.com/v1",
		APIKey:         "sk-resp",
		APIFormat:      formatOpenAIResponses,
		Model:          "gpt-4.1",
		Thinking:       true,
		ThinkingEffort: "low",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	if responses.URL != "https://api.openai.com/v1/responses" {
		t.Fatalf("responses url %s", responses.URL)
	}
	var responsesBody struct {
		Input     []model.ChatMessage `json:"input"`
		Reasoning struct {
			Effort string `json:"effort"`
		} `json:"reasoning"`
	}
	if err := json.Unmarshal(responses.Body, &responsesBody); err != nil {
		t.Fatal(err)
	}
	if responsesBody.Reasoning.Effort != "low" || len(responsesBody.Input) != 2 || responsesBody.Input[0].Role != "system" {
		t.Fatalf("responses body %#v", responsesBody)
	}
	off, err := buildLLMRequest(AgentConfig{
		APIURL:    "https://api.openai.com/v1",
		APIKey:    "sk-resp",
		APIFormat: formatOpenAIResponses,
		Model:     "gpt-4.1",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	assertAbsent(t, off.Body, "reasoning")

	anthropic, err := buildLLMRequest(AgentConfig{
		APIURL:         "https://api.anthropic.com",
		APIKey:         "sk-ant",
		APIFormat:      formatAnthropic,
		Model:          "claude",
		Thinking:       true,
		ThinkingEffort: "medium",
	}, []model.ChatMessage{
		{Role: "system", Content: "一"},
		{Role: "system", Content: "二"},
		{Role: "assistant", Content: "先说"},
		{Role: "user", Content: "甲"},
		{Role: "user", Content: "乙"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if anthropic.URL != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("anthropic url %s", anthropic.URL)
	}
	if anthropic.Header.Get("x-api-key") != "sk-ant" || anthropic.Header.Get("anthropic-version") != anthropicVersion {
		t.Fatalf("anthropic headers %#v", anthropic.Header)
	}
	if anthropic.Header.Get("Authorization") != "" {
		t.Fatal("anthropic must not send bearer")
	}
	var antBody struct {
		System    string             `json:"system"`
		MaxTokens int                `json:"max_tokens"`
		Messages  []anthropicMessage `json:"messages"`
		Thinking  struct {
			Type         string `json:"type"`
			BudgetTokens int    `json:"budget_tokens"`
		} `json:"thinking"`
	}
	if err := json.Unmarshal(anthropic.Body, &antBody); err != nil {
		t.Fatal(err)
	}
	if antBody.System != "一\n\n二" || antBody.MaxTokens != 8192 || antBody.Thinking.BudgetTokens != 4096 || antBody.Thinking.Type != "enabled" {
		t.Fatalf("anthropic body %#v", antBody)
	}
	if len(antBody.Messages) != 3 || antBody.Messages[0].Content != "继续" || antBody.Messages[2].Content != "甲\n\n乙" {
		t.Fatalf("anthropic messages %#v", antBody.Messages)
	}

	plain, err := buildLLMRequest(AgentConfig{
		APIURL:    "https://gateway.example/v1",
		APIKey:    "sk-ant",
		APIFormat: formatAnthropic,
		Model:     "claude",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	if plain.URL != "https://gateway.example/v1/messages" {
		t.Fatalf("anthropic v1 url %s", plain.URL)
	}
	assertAbsent(t, plain.Body, "thinking")
	var plainBody struct {
		MaxTokens int                `json:"max_tokens"`
		Messages  []anthropicMessage `json:"messages"`
	}
	if err := json.Unmarshal(plain.Body, &plainBody); err != nil {
		t.Fatal(err)
	}
	if plainBody.MaxTokens != 8192 || len(plainBody.Messages) != 1 || plainBody.Messages[0].Role != "user" {
		t.Fatalf("plain anthropic %#v", plainBody)
	}

	high, err := buildLLMRequest(AgentConfig{
		APIURL:         "https://api.anthropic.com",
		APIKey:         "sk-ant",
		APIFormat:      formatAnthropic,
		Model:          "claude",
		Thinking:       true,
		ThinkingEffort: "high",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	var highBody struct {
		MaxTokens int `json:"max_tokens"`
		Thinking  struct {
			BudgetTokens int `json:"budget_tokens"`
		} `json:"thinking"`
	}
	if err := json.Unmarshal(high.Body, &highBody); err != nil {
		t.Fatal(err)
	}
	if highBody.Thinking.BudgetTokens != 16000 || highBody.MaxTokens != 20096 {
		t.Fatalf("high budget %#v", highBody)
	}

	xhigh, err := buildLLMRequest(AgentConfig{
		APIURL:         "https://api.openai.com/v1",
		APIKey:         "sk-chat",
		APIFormat:      formatOpenAIChat,
		Model:          "gpt-4o",
		Thinking:       true,
		ThinkingEffort: "xhigh",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	var xhighChat map[string]any
	if err := json.Unmarshal(xhigh.Body, &xhighChat); err != nil {
		t.Fatal(err)
	}
	if xhighChat["reasoning_effort"] != "xhigh" {
		t.Fatalf("xhigh chat %#v", xhighChat)
	}

	xhighAnt, err := buildLLMRequest(AgentConfig{
		APIURL:         "https://api.anthropic.com",
		APIKey:         "sk-ant",
		APIFormat:      formatAnthropic,
		Model:          "claude",
		Thinking:       true,
		ThinkingEffort: "xhigh",
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	var xhighBody struct {
		MaxTokens int `json:"max_tokens"`
		Thinking  struct {
			BudgetTokens int `json:"budget_tokens"`
		} `json:"thinking"`
	}
	if err := json.Unmarshal(xhighAnt.Body, &xhighBody); err != nil {
		t.Fatal(err)
	}
	if xhighBody.Thinking.BudgetTokens != 64000 || xhighBody.MaxTokens != 68096 {
		t.Fatalf("xhigh budget %#v", xhighBody)
	}
}

func TestReadLLMStreamFormats(t *testing.T) {
	cases := []struct {
		format string
		body   string
		want   string
	}{
		{
			format: formatOpenAIChat,
			body:   "data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"reasoning_content\":\"skip\",\"content\":\"B\"}}]}\n\ndata: [DONE]\n\n",
			want:   "AB",
		},
		{
			format: formatOpenAIResponses,
			body:   "event: response.output_text.delta\r\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"A\"}\r\n\r\ndata: {\"type\":\"response.output_text.delta\",\"delta\":{\"text\":\"B\"}}\r\n\r\n",
			want:   "AB",
		},
		{
			format: formatAnthropic,
			body:   "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"nope\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n",
			want:   "Hi",
		},
	}
	for _, tc := range cases {
		var chunks []string
		got, err := readLLMStream(tc.format, strings.NewReader(tc.body), func(chunk string) error {
			chunks = append(chunks, chunk)
			return nil
		})
		if err != nil {
			t.Fatalf("%s: %v", tc.format, err)
		}
		if got != tc.want || strings.Join(chunks, "") != tc.want {
			t.Fatalf("%s got %q chunks %#v", tc.format, got, chunks)
		}
	}

	_, err := readLLMStream(formatAnthropic, strings.NewReader("data: {\"type\":\"error\",\"error\":{\"message\":\"boom\"}}\n\n"), nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("anthropic error: %v", err)
	}
	_, err = readLLMStream(formatOpenAIResponses, strings.NewReader("data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"failed\"}}}\n\n"), nil)
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("responses error: %v", err)
	}
	_, err = readLLMStream(formatOpenAIChat, strings.NewReader("data: not-json\n\n"), nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestListModelsUsesFormatAndStoredKey(t *testing.T) {
	var gotPath, gotAuth, gotVersion, gotKey, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("anthropic-version")
		gotKey = r.Header.Get("x-api-key")
		gotMethod = r.Method
		switch r.URL.Path {
		case "/v1/models":
			if r.Header.Get("x-api-key") != "" {
				w.Write([]byte(`{"data":[{"id":"claude-b"},{"id":"claude-a"},{"id":"claude-a"},{"id":" "}]}`))
				return
			}
			w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
		case "/models":
			w.Write([]byte(`{"models":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
		default:
			http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	svc := &AgentService{}
	svc.config.APIKey = "stored-key-1234"

	ids, err := svc.ListModels(context.Background(), srv.URL+"/v1", formatOpenAIChat, "sk-s****1234")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/v1/models" || gotAuth != "Bearer stored-key-1234" {
		t.Fatalf("openai list method=%s path=%s auth=%s", gotMethod, gotPath, gotAuth)
	}
	if strings.Join(ids, ",") != "gpt-4o,gpt-4o-mini" {
		t.Fatalf("openai ids %#v", ids)
	}

	ids, err = svc.ListModels(context.Background(), srv.URL, formatAnthropic, "sk-live")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/models?limit=100" || gotAuth != "" || gotKey != "sk-live" || gotVersion != anthropicVersion {
		t.Fatalf("anthropic list path=%s auth=%s key=%s version=%s", gotPath, gotAuth, gotKey, gotVersion)
	}
	if strings.Join(ids, ",") != "claude-a,claude-b" {
		t.Fatalf("anthropic ids %#v", ids)
	}

	ids, err = svc.ListModels(context.Background(), srv.URL+"/v1", formatAnthropic, "sk-live")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/models?limit=100" {
		t.Fatalf("anthropic v1 path %s", gotPath)
	}

	_, err = svc.ListModels(context.Background(), "", formatOpenAIChat, "sk-live")
	if err == nil || err.Error() != "请先填写 API URL、格式和 API Key" {
		t.Fatalf("missing url: %v", err)
	}
	_, err = svc.ListModels(context.Background(), srv.URL, "xml", "sk-live")
	if err == nil || err.Error() != "不支持的接口格式" {
		t.Fatalf("bad format: %v", err)
	}

	_, err = svc.ListModels(context.Background(), srv.URL+"/missing", formatOpenAIChat, "sk-live")
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("upstream: %v", err)
	}
}

func TestSaveConfigKeepsKeyAndNormalizes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")
	svc := &AgentService{configPath: path}
	if err := svc.SaveConfig(AgentConfig{
		APIURL:         " https://example.com/v1/ ",
		APIKey:         "sk-secret-key",
		APIFormat:      "nope",
		Model:          " gpt-4o ",
		Enabled:        true,
		Thinking:       true,
		ThinkingEffort: "ultra",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := svc.GetConfig()
	if cfg.APIURL != "https://example.com/v1" || cfg.Model != "gpt-4o" || cfg.APIFormat != formatOpenAIChat || cfg.ThinkingEffort != "medium" || cfg.APIKey != "sk-secret-key" || !cfg.Thinking {
		t.Fatalf("normalized %#v", cfg)
	}
	if err := svc.SaveConfig(AgentConfig{
		APIURL:    "https://example.com/v1",
		APIKey:    svc.MaskedConfig().APIKey,
		APIFormat: formatAnthropic,
		Model:     "claude",
		Enabled:   true,
	}); err != nil {
		t.Fatal(err)
	}
	if svc.GetConfig().APIKey != "sk-secret-key" || svc.GetConfig().APIFormat != formatAnthropic {
		t.Fatalf("masked save %#v", svc.GetConfig())
	}

	raw := []byte(`{"api_url":"https://keep.example/v1","api_key":"abc","model":"m","enabled":true}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	svc.config = AgentConfig{}
	if err := svc.loadConfig(); err != nil {
		t.Fatal(err)
	}
	if svc.GetConfig().APIFormat != formatOpenAIChat || svc.GetConfig().ThinkingEffort != "medium" {
		t.Fatalf("loaded %#v", svc.GetConfig())
	}
	disk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != string(raw) {
		t.Fatalf("load rewrote config: %s", disk)
	}
}

func TestAgentConnectionProbe(t *testing.T) {
	var gotPath, gotAuth, gotKey string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotKey = r.Header.Get("x-api-key")
		raw, _ := io.ReadAll(r.Body)
		gotBody = map[string]any{}
		_ = json.Unmarshal(raw, &gotBody)
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\n"))
		case "/v1/responses":
			w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))
		case "/v1/messages":
			w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n"))
		default:
			http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	svc := &AgentService{}
	svc.config.APIKey = "stored-key-1234"
	reply, err := svc.TestConnection(context.Background(), AgentConfig{
		APIURL:         srv.URL + "/v1",
		APIFormat:      formatOpenAIChat,
		APIKey:         "sk-s****1234",
		Model:          "gpt-test",
		Thinking:       true,
		ThinkingEffort: "xhigh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "pong" || gotPath != "/v1/chat/completions" || gotAuth != "Bearer stored-key-1234" {
		t.Fatalf("chat reply=%q path=%s auth=%s", reply, gotPath, gotAuth)
	}
	if _, ok := gotBody["reasoning_effort"]; ok {
		t.Fatalf("probe sent thinking: %#v", gotBody)
	}
	messages, _ := gotBody["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("probe messages %#v", gotBody["messages"])
	}

	reply, err = svc.TestConnection(context.Background(), AgentConfig{
		APIURL:    srv.URL + "/v1",
		APIFormat: formatOpenAIResponses,
		APIKey:    "sk-resp",
		Model:     "gpt-test",
	})
	if err != nil || reply != "ok" || gotPath != "/v1/responses" {
		t.Fatalf("responses reply=%q path=%s err=%v", reply, gotPath, err)
	}

	reply, err = svc.TestConnection(context.Background(), AgentConfig{
		APIURL:    srv.URL,
		APIFormat: formatAnthropic,
		APIKey:    "sk-ant",
		Model:     "claude",
	})
	if err != nil || reply != "hi" || gotPath != "/v1/messages" || gotKey != "sk-ant" || gotAuth != "" {
		t.Fatalf("anthropic reply=%q path=%s key=%s auth=%s err=%v", reply, gotPath, gotKey, gotAuth, err)
	}

	_, err = svc.TestConnection(context.Background(), AgentConfig{
		APIURL:    srv.URL + "/v1",
		APIFormat: formatOpenAIChat,
		APIKey:    "sk-live",
	})
	if err == nil || err.Error() != "请先填写 API URL、格式、API Key 和模型" {
		t.Fatalf("missing model: %v", err)
	}

	_, err = svc.TestConnection(context.Background(), AgentConfig{
		APIURL:    srv.URL + "/missing",
		APIFormat: formatOpenAIChat,
		APIKey:    "sk-live",
		Model:     "gpt-test",
	})
	if err == nil || !strings.Contains(err.Error(), "连接失败") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("upstream: %v", err)
	}
}

func TestFallbackToolCommand(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{message: "查看哪个容器运行中", want: "list_containers()"},
		{message: "查看有哪个容器运行中", want: "list_containers()"},
		{message: "你能使用什么工具", want: ""},
		{message: "停止 nginx 容器", want: ""},
	}
	for _, tc := range cases {
		if got := fallbackToolCommand(tc.message); got != tc.want {
			t.Fatalf("%q got %q", tc.message, got)
		}
	}
}

type listOnlyDocker struct {
	repository.IDockerRepository
	mu         sync.Mutex
	calls      int
	containers []types.Container
}

func (d *listOnlyDocker) ContainerList(context.Context, types.ContainerListOptions) ([]types.Container, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	return d.containers, nil
}

func TestStreamChatListsContainersWhenModelRefuses(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.Copy(io.Discard, r.Body)
		if calls == 1 {
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"当前这边没有连上你的 Docker 主机\"}}]}\n\n"))
			return
		}
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"当前有 1 个容器正在运行。\"}}]}\n\n"))
	}))
	defer srv.Close()

	dockerStub := &listOnlyDocker{containers: []types.Container{{
		ID:     "bf82a246af3c0000",
		Names:  []string{"/rabbit-panel"},
		Status: "Up 3 minutes",
	}}}
	svc := &AgentService{dockerRepo: dockerStub}
	svc.config = normalizeAgentConfig(AgentConfig{
		APIURL:    srv.URL + "/v1",
		APIKey:    "sk-test",
		APIFormat: formatOpenAIChat,
		Model:     "stub",
		Enabled:   true,
	})

	var got strings.Builder
	err := svc.StreamChat(context.Background(), model.AgentChatRequest{Message: "查看哪个容器运行中"}, func(chunk string) error {
		got.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := got.String()
	if strings.Contains(text, "没有连上") || !strings.Contains(text, "rabbit-panel") || !strings.Contains(text, "正在运行") {
		t.Fatalf("reply %q calls=%d docker=%d", text, calls, dockerStub.calls)
	}
	if dockerStub.calls != 1 || calls != 2 {
		t.Fatalf("docker calls=%d llm calls=%d", dockerStub.calls, calls)
	}
}

func assertAbsent(t *testing.T, raw []byte, key string) {
	t.Helper()
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body[key]; ok {
		t.Fatalf("unexpected %s in %s", key, raw)
	}
}
