package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"rabbit-panel/model"
)

type scriptAgent struct {
	mu     sync.Mutex
	reply  string
	hist   []int
	called int
}

func (a *scriptAgent) Complete(_ context.Context, message string, history []model.ChatMessage) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.called++
	a.hist = append(a.hist, len(history))
	if a.reply != "" {
		return a.reply, nil
	}
	return "pong:" + message, nil
}

type ilinkFake struct {
	mu         sync.Mutex
	mode       string
	qrPolls    int
	updates    int
	verifySeen string
	sends      []string
	contexts   []string
	authSeen   string
	qrAuth     string
	stopped    chan struct{}
	stopOnce   sync.Once
	sendCh     chan struct{}
}

func (f *ilinkFake) handler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ilink/bot/get_bot_qrcode":
		f.mu.Lock()
		f.qrAuth = r.Header.Get("Authorization")
		f.mu.Unlock()
		if r.Header.Get("iLink-App-Id") != "bot" || r.Header.Get("iLink-App-ClientVersion") == "" {
			http.Error(w, "missing ilink headers", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"qrcode":"qr-1","qrcode_img_content":"https://liteapp.weixin.qq.com/q/rabbit"}`))
	case "/ilink/bot/get_qrcode_status":
		f.mu.Lock()
		f.qrPolls++
		n := f.qrPolls
		code := r.URL.Query().Get("verify_code")
		if code != "" {
			f.verifySeen = code
		}
		mode := f.mode
		f.mu.Unlock()
		if mode == "verify" && code != "2468" {
			_, _ = w.Write([]byte(`{"status":"need_verifycode"}`))
			return
		}
		if mode == "login" && n == 1 {
			_, _ = w.Write([]byte(`{"status":"wait"}`))
			return
		}
		base := "http://" + r.Host
		if r.TLS != nil {
			base = "https://" + r.Host
		}
		fmt.Fprintf(w, `{"status":"confirmed","bot_token":"tok-1234567890abcd","ilink_bot_id":"bot-1","ilink_user_id":"user-1","baseurl":"%s"}`, base)
	case "/ilink/bot/getupdates":
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "no auth", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"channel_version":"2.2.0"`) {
			http.Error(w, "no base_info", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.authSeen = r.Header.Get("Authorization")
		f.updates++
		n := f.updates
		mode := f.mode
		f.mu.Unlock()
		if mode == "expire" {
			_, _ = w.Write([]byte(`{"errcode":-14,"errmsg":"session expired"}`))
			return
		}
		if mode == "image" && n == 1 {
			_, _ = w.Write([]byte(`{"ret":0,"get_updates_buf":"c1","msgs":[{"message_id":3,"from_user_id":"user-9","message_type":1,"message_state":2,"context_token":"ctx-img","item_list":[{"type":2}]}]}`))
			return
		}
		if n == 1 {
			_, _ = w.Write([]byte(`{"ret":0,"get_updates_buf":"c1","msgs":[{"message_id":7,"from_user_id":"user-9","message_type":1,"message_state":2,"context_token":"ctx-1","item_list":[{"type":1,"text_item":{"text":"你好"}}]},{"message_id":7,"from_user_id":"user-9","message_type":1,"message_state":2,"context_token":"ctx-1","item_list":[{"type":1,"text_item":{"text":"重复"}}]}]}`))
			return
		}
		if n == 2 && mode == "login" {
			_, _ = w.Write([]byte(`{"ret":0,"get_updates_buf":"c2","msgs":[{"message_id":8,"from_user_id":"user-9","message_type":1,"message_state":2,"context_token":"ctx-1","item_list":[{"type":1,"text_item":{"text":"再问"}}]}]}`))
			return
		}
		<-r.Context().Done()
	case "/ilink/bot/sendmessage":
		var payload struct {
			Msg struct {
				Context string `json:"context_token"`
				Items   []struct {
					Text struct {
						Text string `json:"text"`
					} `json:"text_item"`
				} `json:"item_list"`
			} `json:"msg"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		text := ""
		if len(payload.Msg.Items) > 0 {
			text = payload.Msg.Items[0].Text.Text
		}
		f.mu.Lock()
		f.sends = append(f.sends, text)
		f.contexts = append(f.contexts, payload.Msg.Context)
		f.mu.Unlock()
		select {
		case f.sendCh <- struct{}{}:
		default:
		}
		_, _ = w.Write([]byte(`{"ret":0}`))
	case "/ilink/bot/msg/notifystart":
		_, _ = w.Write([]byte(`{"ret":0}`))
	case "/ilink/bot/msg/notifystop":
		f.stopOnce.Do(func() { close(f.stopped) })
		_, _ = w.Write([]byte(`{"ret":0}`))
	default:
		http.NotFound(w, r)
	}
}

func newILinkService(t *testing.T, mode string) (*WeChatService, *ilinkFake, *scriptAgent) {
	t.Helper()
	fake := &ilinkFake{
		mode:    mode,
		stopped: make(chan struct{}),
		sendCh:  make(chan struct{}, 8),
	}
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(server.Close)
	agent := &scriptAgent{reply: "好的"}
	service := newWeChatService(agent, filepath.Join(t.TempDir(), "wechat.json"))
	service.apiBase = server.URL
	service.qrEvery = 20 * time.Millisecond
	t.Cleanup(func() { _ = service.Unbind() })
	return service, fake, agent
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout")
}

func TestILinkLoginReplyAndRebind(t *testing.T) {
	service, fake, agent := newILinkService(t, "login")
	qr, err := service.StartQR(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(qr.Image, "data:image/png;base64,") {
		t.Fatalf("image = %s", qr.Image[:20])
	}
	waitUntil(t, func() bool { return service.Status().Linked })
	status := service.Status()
	if status.TokenMask != "tok-****abcd" {
		t.Fatalf("mask = %s", status.TokenMask)
	}
	if strings.Contains(status.TokenMask, "1234567890") {
		t.Fatal("raw token leaked")
	}
	waitUntil(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.sends) >= 2
	})
	fake.mu.Lock()
	if fake.qrAuth != "" {
		t.Fatalf("qr request should not send Authorization, got %s", fake.qrAuth)
	}
	if fake.authSeen != "Bearer tok-1234567890abcd" {
		t.Fatalf("auth = %s", fake.authSeen)
	}
	if fake.contexts[0] != "ctx-1" || fake.sends[0] != "好的" || fake.sends[1] != "好的" {
		t.Fatalf("sends=%v contexts=%v", fake.sends, fake.contexts)
	}
	fake.mu.Unlock()
	agent.mu.Lock()
	if agent.called != 2 || agent.hist[0] != 0 || agent.hist[1] != 2 {
		t.Fatalf("agent called=%d hist=%v", agent.called, agent.hist)
	}
	agent.mu.Unlock()

	if _, err := service.StartQR(t.Context()); err == nil || !strings.Contains(err.Error(), "解除绑定") {
		t.Fatalf("second qr err = %v", err)
	}
	if err := service.Unbind(); err != nil {
		t.Fatal(err)
	}
	if service.Status().Linked {
		t.Fatal("still linked")
	}
	select {
	case <-fake.stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("notifystop was not called")
	}
	data, err := os.ReadFile(service.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "tok-1234567890abcd") {
		t.Fatal("token still on disk")
	}
}

func TestILinkVerifyCode(t *testing.T) {
	service, fake, _ := newILinkService(t, "verify")
	if _, err := service.StartQR(t.Context()); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return service.QR().Status == "verify" })
	if err := service.SubmitVerify("2468"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return service.Status().Linked })
	fake.mu.Lock()
	seen := fake.verifySeen
	fake.mu.Unlock()
	if seen != "2468" {
		t.Fatalf("verify code = %s", seen)
	}
}

func TestILinkImageOnly(t *testing.T) {
	service, fake, agent := newILinkService(t, "image")
	service.mu.Lock()
	service.store.Token = "tok-1234567890abcd"
	service.store.BaseURL = service.apiBase
	service.mu.Unlock()
	service.restore()
	waitUntil(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.sends) >= 1
	})
	fake.mu.Lock()
	text := fake.sends[0]
	fake.mu.Unlock()
	if text != "目前只支持文字消息。" {
		t.Fatalf("image reply = %s", text)
	}
	agent.mu.Lock()
	called := agent.called
	agent.mu.Unlock()
	if called != 0 {
		t.Fatalf("agent called for image: %d", called)
	}
}

func TestILinkSessionExpired(t *testing.T) {
	service, _, _ := newILinkService(t, "expire")
	service.mu.Lock()
	service.store.Token = "tok-1234567890abcd"
	service.store.BaseURL = service.apiBase
	service.mu.Unlock()
	service.restore()
	waitUntil(t, func() bool {
		status := service.Status()
		return !status.Linked && status.SessionExpired
	})
}

func TestILinkDiscardOfficialAccountConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wechat.json")
	if err := os.WriteFile(path, []byte(`{"app_id":"wx1","app_secret":"secret-value","token":"rabbit"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newWeChatService(nil, path)
	if service.store.Token != "" {
		t.Fatalf("kept old token %s", service.store.Token)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-value") || strings.Contains(string(data), "wx1") {
		t.Fatalf("old secret remains: %s", data)
	}
}

func TestMaskAndChunkWeChatText(t *testing.T) {
	if maskSecret("") != "" || maskSecret("short") != "****" || maskSecret("123456789abc") != "1234****9abc" {
		t.Fatalf("mask %q %q %q", maskSecret(""), maskSecret("short"), maskSecret("123456789abc"))
	}
	if parts := chunkWeChatText("你好", 2000); len(parts) != 1 || parts[0] != "你好" {
		t.Fatalf("short = %#v", parts)
	}
	parts := chunkWeChatText(strings.Repeat("啊", 4500), 2000)
	if len(parts) != 3 || len([]rune(parts[0])) != 2000 || len([]rune(parts[2])) != 500 {
		t.Fatalf("parts=%d", len(parts))
	}
}
