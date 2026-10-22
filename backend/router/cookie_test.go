package router

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	config2 "rabbit-panel/config"
)

// 登录态 Cookie 的 Secure 标志必须跟随实际连接协议。
func TestSetAuthCookieSecureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name      string
		tlsOn     bool
		reqTLS    bool
		fwdProto  string
		wantSecur bool
	}{
		{name: "原生 HTTPS", tlsOn: true, wantSecur: true},
		{name: "反向代理终止 TLS", tlsOn: false, fwdProto: "https", wantSecur: true},
		{name: "请求本身是 TLS", tlsOn: false, reqTLS: true, wantSecur: true},
		{name: "纯 HTTP", tlsOn: false, wantSecur: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := &config2.App{}
			app.TLS.Enabled = c.tlsOn
			r := &Router{app: app}

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			if c.reqTLS {
				req.TLS = &tls.ConnectionState{}
			}
			if c.fwdProto != "" {
				req.Header.Set("X-Forwarded-Proto", c.fwdProto)
			}

			rec := httptest.NewRecorder()
			c2, _ := gin.CreateTestContext(rec)
			c2.Request = req

			r.setAuthCookie(c2, "token-value", 86400)

			raw := rec.Header().Get("Set-Cookie")
			if raw == "" {
				t.Fatal("Set-Cookie header missing")
			}
			if !strings.Contains(raw, "token=token-value") {
				t.Fatalf("unexpected cookie value: %s", raw)
			}
			hasSecure := strings.Contains(raw, "; Secure")
			if hasSecure != c.wantSecur {
				t.Fatalf("Secure flag = %v, want %v (cookie: %s)", hasSecure, c.wantSecur, raw)
			}
			if !strings.Contains(raw, "HttpOnly") {
				t.Fatalf("cookie should stay HttpOnly: %s", raw)
			}
		})
	}
}
