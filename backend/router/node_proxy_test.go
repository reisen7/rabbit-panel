package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/gin-gonic/gin"

	config2 "rabbit-panel/config"
	"rabbit-panel/middleware"
	"rabbit-panel/model"
	"rabbit-panel/repository"
	"rabbit-panel/service"
)

type proxyTestDocker struct {
	repository.IDockerRepository
}

func (d *proxyTestDocker) ContainerList(context.Context, types.ContainerListOptions) ([]types.Container, error) {
	return nil, nil
}

// 安全属性：Master 代理到节点的请求绝不携带用户凭证（JWT/Cookie/query token），
// 代之以一次性时间戳节点令牌。恶意注册节点无法借此窃取管理员身份。
func TestNodeProxyStripsUserCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const nodeSecret = "proxy-test-secret-0123456789abcdef"
	jwtSecret := []byte("proxy-test-jwt-secret-0123456789ab")

	var gotAuth, gotCookie, gotQuery, gotNodeID, gotNodeToken string
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotQuery = r.URL.RawQuery
		gotNodeID = r.Header.Get("X-Node-ID")
		gotNodeToken = r.Header.Get("X-Node-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer evil.Close()
	evilAddr := strings.TrimPrefix(evil.URL, "http://")

	app := &config2.App{
		Mode:       "master",
		JWTSecret:  jwtSecret,
		NodeSecret: nodeSecret,
	}
	app.NodeService = service.NewNodeService(&proxyTestDocker{}, nil, "master", nodeSecret, "local:3958", "127.0.0.1", "3958")
	app.NodeService.RegisterNode(&model.NodeInfo{
		ID:      "evil:3958",
		Name:    "evil",
		Address: evilAddr,
		Mode:    model.ModeWorker,
		Status:  model.NodeStatusOnline,
	})

	r := &Router{app: app}
	engine := gin.New()
	engine.Use(r.containerNodeProxy())
	// 反向代理依赖 CloseNotifier，httptest.ResponseRecorder 不支持，必须用真实服务器
	proxySrv := httptest.NewServer(engine)
	defer proxySrv.Close()

	userToken, err := middleware.GenerateToken("admin", false, jwtSecret)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodGet,
		proxySrv.URL+"/api/containers?target_node=evil:3958&token="+userToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Cookie", "token="+userToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("proxy status %d", resp.StatusCode)
	}
	if gotAuth != "" {
		t.Fatalf("user Authorization header leaked to node: %q", gotAuth)
	}
	if gotCookie != "" {
		t.Fatalf("user Cookie leaked to node: %q", gotCookie)
	}
	if strings.Contains(gotQuery, "token=") {
		t.Fatalf("user token query param leaked to node: %q", gotQuery)
	}
	if strings.Contains(gotQuery, "target_node=") {
		t.Fatalf("target_node param should be stripped: %q", gotQuery)
	}
	if gotNodeID != "master" || !middleware.ValidateNodeToken(gotNodeID, gotNodeToken, nodeSecret) {
		t.Fatalf("expected valid node credentials, got id=%q token=%q", gotNodeID, gotNodeToken)
	}
}

// 未携带有效用户凭证的请求不得进入代理转发。
func TestNodeProxyRejectsUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	app := &config2.App{
		Mode:       "master",
		JWTSecret:  []byte("proxy-test-jwt-secret-0123456789ab"),
		NodeSecret: "proxy-test-secret-0123456789abcdef",
	}
	app.NodeService = service.NewNodeService(&proxyTestDocker{}, nil, "master", app.NodeSecret, "local:3958", "127.0.0.1", "3958")

	r := &Router{app: app}
	engine := gin.New()
	engine.Use(r.containerNodeProxy())

	req := httptest.NewRequest(http.MethodGet, "/api/containers?target_node=evil:3958", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
