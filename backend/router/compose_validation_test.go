package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	config2 "rabbit-panel/config"
	"rabbit-panel/middleware"
	"rabbit-panel/repository"
	"rabbit-panel/service"
)

var composeTestJWT = []byte("0123456789abcdef0123456789abcdef")

// newComposeTestEngine 用真实 FileRepository（t.TempDir 隔离 ./compose_projects）
// 与 stub docker 仓库组装 compose 路由，和线上一样走用户 JWT 认证。
func newComposeTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	app := &config2.App{JWTSecret: composeTestJWT}
	app.ComposeService = service.NewComposeService(&proxyTestDocker{}, repository.NewFileRepository())
	r := &Router{app: app}

	engine := gin.New()
	api := engine.Group("/api")
	api.Use(middleware.AuthMiddleware(app.JWTSecret, "/api/auth/login", "/api/health"))
	api.POST("/compose/create", r.handleComposeCreate)
	api.GET("/compose/file", r.handleComposeFile)
	api.POST("/compose/file", r.handleComposeFile)
	api.POST("/compose/save", r.handleComposeSaveFile)
	api.POST("/compose/action", r.handleComposeAction)
	api.POST("/compose/delete", r.handleComposeDelete)
	api.GET("/compose/status", r.handleComposeStatus)
	return engine
}

func composeAuthRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	token, err := middleware.GenerateToken("admin", false, composeTestJWT)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// 恶意项目名必须在进入 service 之前被 400 拒绝（而非 500/SSE 流内错误），
// 错误信息含"项目名"便于前端展示与排查。
func TestComposeEndpointsRejectTraversalProjectName(t *testing.T) {
	engine := newComposeTestEngine(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"create", http.MethodPost, "/api/compose/create", map[string]any{"name": "../x", "content": "x"}},
		{"file GET query", http.MethodGet, "/api/compose/file?project=..%2fx", nil},
		{"file POST body", http.MethodPost, "/api/compose/file", map[string]any{"project": "../x", "content": "x"}},
		{"file POST query", http.MethodPost, "/api/compose/file?project=..%2fx", map[string]any{"content": "x"}},
		{"save POST body", http.MethodPost, "/api/compose/save", map[string]any{"project": "../x", "content": "x"}},
		{"action", http.MethodPost, "/api/compose/action", map[string]any{"project": "../x", "action": "up"}},
		{"delete", http.MethodPost, "/api/compose/delete", map[string]any{"project": "../x"}},
		{"status", http.MethodGet, "/api/compose/status?project=..%2fx", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, composeAuthRequest(t, tc.method, tc.path, tc.body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "项目名") {
				t.Fatalf("body 应含项目名校验错误, got %s", rec.Body.String())
			}
			// action 端点在拒绝时必须是普通 JSON，不得已进入 SSE 流
			if tc.name == "action" {
				if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "event-stream") {
					t.Fatalf("action 拒绝时不得使用 SSE 响应, content-type = %s", ct)
				}
			}
		})
	}
}

// 合法项目名必须能通过校验：create/file/status/delete 走通完整流程，
// action 至少不返回 400（docker 二进制在测试机未必可用，错误进 SSE 流属预期）。
func TestComposeEndpointsAcceptValidProjectName(t *testing.T) {
	engine := newComposeTestEngine(t)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, composeAuthRequest(t, http.MethodPost, "/api/compose/create",
		map[string]any{"name": "valid-proj", "content": "services: {}\n"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("create code = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, composeAuthRequest(t, http.MethodGet, "/api/compose/file?project=valid-proj", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("file GET code = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "services") {
		t.Fatalf("file content unexpected: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, composeAuthRequest(t, http.MethodGet, "/api/compose/status?project=valid-proj", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", rec.Code, rec.Body.String())
	}

	// action 对合法项目名不得 400。用未知 action 保持测试闭环：
	// 项目名通过校验后进入 SSE 流（200 + event-stream），错误在流内而非状态码，
	// 且不依赖测试机上是否存在 docker 二进制。
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, composeAuthRequest(t, http.MethodPost, "/api/compose/action",
		map[string]any{"project": "valid-proj", "action": "bogus"}))
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("action 对合法项目名返回 400: %s", rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("action code = %d, want 200(SSE); body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "event-stream") {
		t.Fatalf("合法项目名 action 应进入 SSE 流, content-type = %s", ct)
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, composeAuthRequest(t, http.MethodPost, "/api/compose/delete",
		map[string]any{"project": "valid-proj"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code = %d, body = %s", rec.Code, rec.Body.String())
	}
}
