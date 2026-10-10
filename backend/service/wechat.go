package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"

	"rabbit-panel/model"
)

const (
	defaultILinkBase    = "https://ilinkai.weixin.qq.com"
	ilinkChannelVersion = "2.2.0"
	// iLink-App-ClientVersion：把 channel_version 编成 0x00MMNNPP。2.2.0 → 131584。
	ilinkClientVersion = "131584"
	wechatTextLimit    = 2000
	wechatHistoryLimit = 20
)

var errSessionExpired = errors.New("wechat session expired")

// agentRunner 是智能体的同步对话入口。*AgentService 已实现它。
type agentRunner interface {
	Complete(ctx context.Context, message string, history []model.ChatMessage) (string, error)
}

// WeChatStatus 是设置页看到的连接状态。Token 已脱敏。
type WeChatStatus struct {
	Linked         bool      `json:"linked"`
	TokenMask      string    `json:"token_mask"`
	AccountID      string    `json:"account_id"`
	SavedAt        string    `json:"saved_at"`
	Online         bool      `json:"online"`
	SessionExpired bool      `json:"session_expired"`
	QR             *WeChatQR `json:"qr,omitempty"`
}

// WeChatQR 是当前这张登录二维码。
type WeChatQR struct {
	Image   string `json:"image,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type wechatStore struct {
	Token         string                         `json:"token,omitempty"`
	BaseURL       string                         `json:"base_url,omitempty"`
	AccountID     string                         `json:"account_id,omitempty"`
	UserID        string                         `json:"user_id,omitempty"`
	SavedAt       string                         `json:"saved_at,omitempty"`
	Cursor        string                         `json:"cursor,omitempty"`
	ContextTokens map[string]string              `json:"context_tokens,omitempty"`
	History       map[string][]model.ChatMessage `json:"history,omitempty"`
}

type qrCreateBody struct {
	QRCode string `json:"qrcode"`
	Img    string `json:"qrcode_img_content"`
}

type qrStatusBody struct {
	Status       string `json:"status"`
	BotToken     string `json:"bot_token"`
	ILinkBotID   string `json:"ilink_bot_id"`
	ILinkUserID  string `json:"ilink_user_id"`
	BaseURL      string `json:"baseurl"`
	RedirectHost string `json:"redirect_host"`
}

type wireMessage struct {
	MessageID    int64      `json:"message_id"`
	FromUserID   string     `json:"from_user_id"`
	ToUserID     string     `json:"to_user_id"`
	MessageType  int        `json:"message_type"`
	MessageState int        `json:"message_state"`
	ContextToken string     `json:"context_token"`
	GroupID      string     `json:"group_id"`
	ItemList     []wireItem `json:"item_list"`
}

type wireItem struct {
	Type      int            `json:"type"`
	TextItem  *wireTextItem  `json:"text_item"`
	VoiceItem *wireVoiceItem `json:"voice_item"`
}

type wireTextItem struct {
	Text string `json:"text"`
}

type wireVoiceItem struct {
	Text string `json:"text"`
}

type updatesBody struct {
	Msgs []wireMessage `json:"msgs"`
	Buf  string        `json:"get_updates_buf"`
}

// WeChatService 用 iLink Bot API 把微信消息接到智能体。
// 扫码登录拿到 bot_token，长轮询 getupdates，再用入站 context_token 回复。
type WeChatService struct {
	agent      agentRunner
	path       string
	apiBase    string
	qrEvery    time.Duration
	httpClient *http.Client

	mu             sync.Mutex
	store          wechatStore
	sessionExpired bool
	qr             WeChatQR
	qrGen          int
	qrCancel       context.CancelFunc
	verifyCode     string
	verifyWait     chan struct{}
	seen           map[int64]time.Time

	pollMu     sync.Mutex
	pollCancel context.CancelFunc
	pollLoopMu sync.Mutex
	online     bool
}

// NewWeChatService 创建微信通道。凭证保存在 data/wechat.json，重启后不用重新扫码。
func NewWeChatService(agent *AgentService) *WeChatService {
	service := newWeChatService(agent, "./data/wechat.json")
	service.restore()
	return service
}

func newWeChatService(agent agentRunner, path string) *WeChatService {
	service := &WeChatService{
		agent:      agent,
		path:       path,
		apiBase:    defaultILinkBase,
		qrEvery:    2 * time.Second,
		httpClient: &http.Client{},
		store:      newWeChatStore(),
		seen:       map[int64]time.Time{},
	}
	service.load()
	return service
}

func newWeChatStore() wechatStore {
	return wechatStore{
		ContextTokens: map[string]string{},
		History:       map[string][]model.ChatMessage{},
	}
}

func (s *WeChatService) restore() {
	s.mu.Lock()
	linked := s.store.Token != ""
	s.mu.Unlock()
	if linked {
		s.startPoller()
	}
}

// Status 返回设置页需要的状态。
func (s *WeChatService) Status() WeChatStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	linked := s.store.Token != ""
	status := WeChatStatus{
		Linked:         linked,
		TokenMask:      maskSecret(s.store.Token),
		AccountID:      s.store.AccountID,
		SavedAt:        s.store.SavedAt,
		Online:         s.online,
		SessionExpired: s.sessionExpired && !linked,
	}
	if !linked && s.qr.Status != "" {
		qr := s.qr
		status.QR = &qr
	}
	return status
}

// QR 返回当前二维码的扫码进度。
func (s *WeChatService) QR() WeChatQR {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.qr.Status == "" {
		return WeChatQR{Status: "idle"}
	}
	return s.qr
}

// StartQR 向微信要一张登录二维码，并在后台等用户扫码确认。
func (s *WeChatService) StartQR(ctx context.Context) (WeChatQR, error) {
	s.mu.Lock()
	if s.store.Token != "" {
		s.mu.Unlock()
		return WeChatQR{}, errors.New("已绑定，请先解除绑定")
	}
	s.mu.Unlock()

	var created qrCreateBody
	err := s.postJSON(ctx, s.apiBase, "/ilink/bot/get_bot_qrcode?bot_type=3", map[string]any{
		"local_token_list": []string{},
	}, "", 15*time.Second, &created)
	if err != nil {
		return WeChatQR{}, fmt.Errorf("获取二维码失败：%s", err.Error())
	}
	if created.QRCode == "" || created.Img == "" {
		return WeChatQR{}, errors.New("微信没有返回二维码")
	}
	png, err := qrcode.Encode(created.Img, qrcode.Medium, 256)
	if err != nil {
		return WeChatQR{}, errors.New("生成二维码图片失败")
	}
	view := WeChatQR{
		Image:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		Status: "pending",
	}

	s.mu.Lock()
	if s.store.Token != "" {
		s.mu.Unlock()
		return WeChatQR{}, errors.New("已绑定，请先解除绑定")
	}
	s.qrGen++
	gen := s.qrGen
	if s.qrCancel != nil {
		s.qrCancel()
	}
	qrCtx, cancel := context.WithCancel(context.Background())
	s.qrCancel = cancel
	s.qr = view
	s.sessionExpired = false
	s.mu.Unlock()

	go s.watchQR(qrCtx, gen, created.QRCode, strings.TrimRight(s.apiBase, "/"))
	return view, nil
}

// SubmitVerify 提交微信配对验证码。
func (s *WeChatService) SubmitVerify(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("请输入验证码")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.qr.Status != "verify" || s.verifyWait == nil {
		return errors.New("当前不需要验证码")
	}
	s.verifyCode = code
	close(s.verifyWait)
	s.verifyWait = nil
	return nil
}

// Unbind 清除登录凭证并停止收消息。
func (s *WeChatService) Unbind() error {
	s.mu.Lock()
	s.qrGen++
	cancelQR := s.qrCancel
	s.qrCancel = nil
	s.qr = WeChatQR{}
	s.verifyWait = nil
	s.store = newWeChatStore()
	s.sessionExpired = false
	s.seen = map[int64]time.Time{}
	err := s.persistLocked()
	s.mu.Unlock()
	if cancelQR != nil {
		cancelQR()
	}
	s.stopPoller()
	return err
}

func (s *WeChatService) watchQR(ctx context.Context, gen int, qrcodeToken, pollBase string) {
	verify := ""
	fails := 0
	for {
		if ctx.Err() != nil || !s.qrCurrent(gen) {
			return
		}
		status, err := s.pollQR(ctx, pollBase, qrcodeToken, verify)
		verify = ""
		if ctx.Err() != nil || !s.qrCurrent(gen) {
			return
		}
		if err != nil {
			fails++
			if fails >= 5 {
				s.setQRStatus(gen, "error", "连接微信失败")
				return
			}
			if !sleepCtx(ctx, s.qrEvery) {
				return
			}
			continue
		}
		fails = 0
		switch status.Status {
		case "wait", "":
			s.setQRStatus(gen, "pending", "")
		case "scaned":
			s.setQRStatus(gen, "scanned", "")
		case "confirmed":
			if err := s.finishLogin(status); err != nil {
				s.setQRStatus(gen, "error", err.Error())
				return
			}
			s.setQRStatus(gen, "linked", "")
			return
		case "expired":
			s.setQRStatus(gen, "expired", "")
			return
		case "need_verifycode":
			s.armVerify(gen)
			code, ok := s.waitVerify(ctx)
			if !ok || !s.qrCurrent(gen) {
				return
			}
			verify = code
			continue
		case "verify_code_blocked":
			s.setQRStatus(gen, "blocked", "")
			return
		case "scaned_but_redirect":
			if host := strings.TrimSpace(status.RedirectHost); host != "" {
				host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
				pollBase = "https://" + host
			}
			s.setQRStatus(gen, "scanned", "")
		case "binded_redirect":
			s.setQRStatus(gen, "error", "这个微信已经绑定在本机，请先解除绑定")
			return
		default:
			s.setQRStatus(gen, "pending", "")
		}
		if !sleepCtx(ctx, s.qrEvery) {
			return
		}
	}
}

func (s *WeChatService) pollQR(ctx context.Context, base, qrcodeToken, verify string) (qrStatusBody, error) {
	path := "/ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcodeToken)
	if verify != "" {
		path += "&verify_code=" + url.QueryEscape(verify)
	}
	var status qrStatusBody
	err := s.getJSON(ctx, base, path, 15*time.Second, &status)
	return status, err
}

func (s *WeChatService) finishLogin(status qrStatusBody) error {
	if status.BotToken == "" || status.ILinkBotID == "" || status.ILinkUserID == "" {
		return errors.New("登录成功但微信没有返回凭证")
	}
	base := strings.TrimRight(status.BaseURL, "/")
	if base == "" {
		base = strings.TrimRight(s.apiBase, "/")
	}
	s.mu.Lock()
	s.store.Token = status.BotToken
	s.store.BaseURL = base
	s.store.AccountID = status.ILinkBotID
	s.store.UserID = status.ILinkUserID
	s.store.SavedAt = time.Now().Format("2006-01-02 15:04")
	s.store.Cursor = ""
	s.store.ContextTokens = map[string]string{}
	s.store.History = map[string][]model.ChatMessage{}
	s.sessionExpired = false
	err := s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	log.Printf("[WeChat] linked account %s", status.ILinkBotID)
	s.startPoller()
	return nil
}

func (s *WeChatService) startPoller() {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	if s.pollCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.pollCancel = cancel
	go s.pollLoop(ctx)
}

func (s *WeChatService) stopPoller() {
	s.pollMu.Lock()
	cancel := s.pollCancel
	s.pollCancel = nil
	s.pollMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *WeChatService) pollLoop(ctx context.Context) {
	s.pollLoopMu.Lock()
	defer s.pollLoopMu.Unlock()

	s.mu.Lock()
	s.online = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.online = false
		s.mu.Unlock()
	}()

	base, token, _ := s.creds()
	s.notify(ctx, base, token, "/ilink/bot/msg/notifystart")
	defer s.notify(context.Background(), base, token, "/ilink/bot/msg/notifystop")

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		base, token, cursor := s.creds()
		if token == "" {
			return
		}
		var updates updatesBody
		err := s.postJSON(ctx, base, "/ilink/bot/getupdates", map[string]any{
			"get_updates_buf": cursor,
			"base_info":       s.baseInfo(),
		}, token, 40*time.Second, &updates)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		if err != nil {
			if errors.Is(err, errSessionExpired) {
				s.expireSession()
				return
			}
			log.Printf("[WeChat] getupdates: %v", err)
			if !sleepCtx(ctx, backoff) {
				return
			}
			if backoff < 10*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		if updates.Buf != "" && updates.Buf != cursor {
			s.setCursor(updates.Buf)
		}
		for _, msg := range updates.Msgs {
			if ctx.Err() != nil {
				return
			}
			s.handleIncoming(ctx, msg)
		}
	}
}

func (s *WeChatService) expireSession() {
	s.mu.Lock()
	s.store = newWeChatStore()
	s.sessionExpired = true
	s.qr = WeChatQR{}
	s.seen = map[int64]time.Time{}
	if err := s.persistLocked(); err != nil {
		log.Printf("[WeChat] clear session: %v", err)
	}
	s.mu.Unlock()
	s.stopPoller()
	log.Printf("[WeChat] iLink session expired")
}

func (s *WeChatService) handleIncoming(ctx context.Context, msg wireMessage) {
	userID := msg.FromUserID
	if msg.MessageType == 2 {
		userID = msg.ToUserID
	}
	if userID == "" {
		return
	}
	if msg.ContextToken != "" {
		s.rememberContext(userID, msg.ContextToken)
	}
	if msg.MessageType != 1 || msg.GroupID != "" || msg.MessageState == 1 {
		return
	}
	if msg.MessageID != 0 && !s.markSeen(msg.MessageID) {
		return
	}
	text := messageText(msg)
	if strings.TrimSpace(text) == "" {
		_ = s.reply(ctx, userID, "目前只支持文字消息。")
		return
	}
	if s.agent == nil {
		_ = s.reply(ctx, userID, "智能体暂时无法回复，请稍后再试。")
		return
	}
	reply, err := s.agent.Complete(ctx, text, s.history(userID))
	if err != nil {
		log.Printf("[WeChat] agent: %v", err)
		_ = s.reply(ctx, userID, "智能体暂时无法回复，请稍后再试。")
		return
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		reply = "（没有生成回复）"
	}
	if err := s.reply(ctx, userID, reply); err != nil {
		log.Printf("[WeChat] send: %v", err)
		return
	}
	s.appendHistory(userID, text, reply)
}

func (s *WeChatService) reply(ctx context.Context, userID, text string) error {
	base, token, contextToken := s.replyCreds(userID)
	if token == "" || contextToken == "" {
		return errors.New("缺少会话凭证")
	}
	for _, chunk := range chunkWeChatText(text, wechatTextLimit) {
		body := map[string]any{
			"msg": map[string]any{
				"from_user_id":  "",
				"to_user_id":    userID,
				"client_id":     uuid.NewString(),
				"message_type":  2,
				"message_state": 2,
				"context_token": contextToken,
				"item_list": []any{
					map[string]any{"type": 1, "text_item": map[string]string{"text": chunk}},
				},
			},
			"base_info": s.baseInfo(),
		}
		if err := s.postJSON(ctx, base, "/ilink/bot/sendmessage", body, token, 15*time.Second, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *WeChatService) notify(ctx context.Context, base, token, path string) {
	if token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.postJSON(ctx, base, path, map[string]any{"base_info": s.baseInfo()}, token, 10*time.Second, nil); err != nil {
		log.Printf("[WeChat] %s: %v", path, err)
	}
}

func (s *WeChatService) baseInfo() map[string]string {
	return map[string]string{
		"channel_version": ilinkChannelVersion,
		"bot_agent":       "RabbitPanel/2.3.0",
	}
}

func (s *WeChatService) postJSON(ctx context.Context, base, path string, body any, token string, timeout time.Duration, out any) error {
	return s.do(ctx, http.MethodPost, base, path, body, token, timeout, out)
}

func (s *WeChatService) getJSON(ctx context.Context, base, path string, timeout time.Duration, out any) error {
	return s.do(ctx, http.MethodGet, base, path, nil, "", timeout, out)
}

func (s *WeChatService) do(ctx context.Context, method, base, path string, body any, token string, timeout time.Duration, out any) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("iLink-App-Id", "bot")
	req.Header.Set("iLink-App-ClientVersion", ilinkClientVersion)
	if token != "" {
		req.Header.Set("AuthorizationType", "ilink_bot_token")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-WECHAT-UIN", randomWeChatUIN())
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("微信接口 HTTP %d", resp.StatusCode)
	}
	if err := checkILink(payload); err != nil {
		return err
	}
	if out != nil && len(bytes.TrimSpace(payload)) > 0 {
		return json.Unmarshal(payload, out)
	}
	return nil
}

func checkILink(payload []byte) error {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	var env struct {
		Ret     *int   `json:"ret"`
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	code := 0
	if env.ErrCode != nil && *env.ErrCode != 0 {
		code = *env.ErrCode
	} else if env.Ret != nil && *env.Ret != 0 {
		code = *env.Ret
	}
	if code == 0 {
		return nil
	}
	if code == -14 {
		return errSessionExpired
	}
	if env.ErrMsg == "" {
		return fmt.Errorf("微信接口错误 %d", code)
	}
	return errors.New(env.ErrMsg)
}

func (s *WeChatService) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("[WeChat] load: %v", err)
		}
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("[WeChat] load: %v", err)
		return
	}
	if _, old := raw["app_id"]; old {
		log.Printf("[WeChat] discard official-account config")
		s.store = newWeChatStore()
		if err := s.persistLocked(); err != nil {
			log.Printf("[WeChat] discard: %v", err)
		}
		return
	}
	if err := json.Unmarshal(data, &s.store); err != nil {
		log.Printf("[WeChat] load: %v", err)
		return
	}
	if s.store.ContextTokens == nil {
		s.store.ContextTokens = map[string]string{}
	}
	if s.store.History == nil {
		s.store.History = map[string][]model.ChatMessage{}
	}
}

func (s *WeChatService) persistLocked() error {
	if err := os.MkdirAll(dirOf(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *WeChatService) creds() (string, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	base := s.store.BaseURL
	if base == "" {
		base = s.apiBase
	}
	return base, s.store.Token, s.store.Cursor
}

func (s *WeChatService) replyCreds(userID string) (string, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	base := s.store.BaseURL
	if base == "" {
		base = s.apiBase
	}
	return base, s.store.Token, s.store.ContextTokens[userID]
}

func (s *WeChatService) setCursor(cursor string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store.Cursor == cursor {
		return
	}
	s.store.Cursor = cursor
	if err := s.persistLocked(); err != nil {
		log.Printf("[WeChat] save cursor: %v", err)
	}
}

func (s *WeChatService) rememberContext(userID, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store.ContextTokens == nil {
		s.store.ContextTokens = map[string]string{}
	}
	if s.store.ContextTokens[userID] == token {
		return
	}
	s.store.ContextTokens[userID] = token
	if err := s.persistLocked(); err != nil {
		log.Printf("[WeChat] save context: %v", err)
	}
}

func (s *WeChatService) history(userID string) []model.ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.store.History[userID]
	out := make([]model.ChatMessage, len(src))
	copy(out, src)
	return out
}

func (s *WeChatService) appendHistory(userID, userText, reply string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store.History == nil {
		s.store.History = map[string][]model.ChatMessage{}
	}
	items := append(s.store.History[userID],
		model.ChatMessage{Role: "user", Content: userText},
		model.ChatMessage{Role: "assistant", Content: reply},
	)
	if len(items) > wechatHistoryLimit {
		items = items[len(items)-wechatHistoryLimit:]
	}
	s.store.History[userID] = items
	if err := s.persistLocked(); err != nil {
		log.Printf("[WeChat] save history: %v", err)
	}
}

func (s *WeChatService) markSeen(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if len(s.seen) > 256 {
		for key, at := range s.seen {
			if now.Sub(at) > 10*time.Minute {
				delete(s.seen, key)
			}
		}
	}
	if _, ok := s.seen[id]; ok {
		return false
	}
	s.seen[id] = now
	return true
}

func (s *WeChatService) qrCurrent(gen int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.qrGen == gen
}

func (s *WeChatService) setQRStatus(gen int, status, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.qrGen != gen {
		return
	}
	s.qr.Status = status
	s.qr.Message = message
}

func (s *WeChatService) armVerify(gen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.qrGen != gen {
		return
	}
	s.qr.Status = "verify"
	s.qr.Message = ""
	s.verifyCode = ""
	s.verifyWait = make(chan struct{})
}

func (s *WeChatService) waitVerify(ctx context.Context) (string, bool) {
	s.mu.Lock()
	ch := s.verifyWait
	s.mu.Unlock()
	if ch == nil {
		return "", false
	}
	select {
	case <-ctx.Done():
		return "", false
	case <-ch:
	}
	s.mu.Lock()
	code := s.verifyCode
	s.verifyCode = ""
	s.mu.Unlock()
	return code, code != ""
}

func messageText(msg wireMessage) string {
	var parts []string
	for _, item := range msg.ItemList {
		if item.Type == 1 && item.TextItem != nil {
			if text := strings.TrimSpace(item.TextItem.Text); text != "" {
				parts = append(parts, text)
			}
		}
		if item.Type == 3 && item.VoiceItem != nil {
			if text := strings.TrimSpace(item.VoiceItem.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func chunkWeChatText(text string, limit int) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	if limit <= 0 || len(runes) <= limit {
		return []string{text}
	}
	var parts []string
	for len(runes) > 0 {
		n := limit
		if n > len(runes) {
			n = len(runes)
		}
		parts = append(parts, string(runes[:n]))
		runes = runes[n:]
	}
	return parts
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}

func randomWeChatUIN() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return base64.StdEncoding.EncodeToString([]byte("1"))
	}
	n := binary.BigEndian.Uint32(buf[:])
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(n), 10)))
}

func dirOf(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[:i]
	}
	return "."
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
