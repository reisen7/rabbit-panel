package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	config2 "rabbit-panel/config"
	"rabbit-panel/middleware"
	"rabbit-panel/repository"
)

var favTestJWT = []byte("0123456789abcdef0123456789abcdef")

func newFavTestEngine(t *testing.T) (*gin.Engine, *repository.SQLiteRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	repo, err := repository.NewSQLiteRepository("test.db")
	if err != nil {
		t.Fatalf("创建测试仓库失败: %v", err)
	}
	t.Cleanup(func() { repo.DB().Close() })
	app := &config2.App{SQLiteRepo: repo, JWTSecret: favTestJWT}
	r := &Router{app: app}

	engine := gin.New()
	api := engine.Group("/api")
	api.Use(middleware.AuthMiddleware(app.JWTSecret, "/api/auth/login", "/api/health"))
	api.GET("/image-favorites", r.handleImageFavoritesList)
	api.POST("/image-favorites", r.handleImageFavoriteUpsert)
	api.DELETE("/image-favorites", r.handleImageFavoriteDelete)
	return engine, repo
}

func favAuthRequest(t *testing.T, method, path string, body any) *http.Request {
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
	token, err := middleware.GenerateToken("admin", false, favTestJWT)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func validFavBody() map[string]any {
	return map[string]any{
		"repository_name": "library/php",
		"registry":        "docker.io",
		"pull_domain":     "docker.1ms.run",
		"tag":             "latest",
		"description":     "PHP 官方镜像",
		"logo_url":        "https://static.1ms.run/image/avatar/x",
		"pull_count":      1417356357,
		"star_count":      7890,
		"is_official":     true,
	}
}

func TestImageFavoritesRoundTrip(t *testing.T) {
	engine, _ := newFavTestEngine(t)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodPost, "/api/image-favorites", validFavBody()))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var stored repository.ImageFavoriteRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stored.ID == 0 || stored.RepositoryName != "library/php" || !stored.IsOfficial {
		t.Fatalf("stored = %+v", stored)
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodGet, "/api/image-favorites", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET code = %d", rec.Code)
	}
	var list []repository.ImageFavoriteRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list) != 1 || list[0].RepositoryName != "library/php" {
		t.Fatalf("list = %+v", list)
	}

	// 空 tag 默认 latest
	body := validFavBody()
	body["repository_name"] = "library/nginx"
	body["tag"] = ""
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodPost, "/api/image-favorites", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST empty tag code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var stored2 repository.ImageFavoriteRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &stored2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stored2.Tag != "latest" {
		t.Fatalf("空 tag 应回退为 latest，got %q", stored2.Tag)
	}
}

// 收藏字段会拼进 docker pull 命令，任何 shell 注入字符都必须被 400 拒绝。
func TestImageFavoritesRejectsInjection(t *testing.T) {
	engine, _ := newFavTestEngine(t)

	cases := []struct {
		name  string
		mutate func(b map[string]any)
	}{
		{"分号注入", func(b map[string]any) { b["repository_name"] = "php; rm -rf /" }},
		{"命令替换", func(b map[string]any) { b["repository_name"] = "$(id)" }},
		{"空格", func(b map[string]any) { b["repository_name"] = "a b" }},
		{"路径穿越", func(b map[string]any) { b["repository_name"] = "../x" }},
		{"反引号", func(b map[string]any) { b["repository_name"] = "php`id`" }},
		{"大写名称", func(b map[string]any) { b["repository_name"] = "PHP" }},
		{"空名称", func(b map[string]any) { b["repository_name"] = "" }},
		{"tag 换行", func(b map[string]any) { b["tag"] = "latest\nwhoami" }},
		{"tag 空格", func(b map[string]any) { b["tag"] = "a b" }},
		{"tag 反引号", func(b map[string]any) { b["tag"] = "x`id`" }},
		{"pull_domain 注入", func(b map[string]any) { b["pull_domain"] = "evil.com;id" }},
		{"pull_domain 换行", func(b map[string]any) { b["pull_domain"] = "evil.com\nid" }},
		{"pull_domain 空格", func(b map[string]any) { b["pull_domain"] = "ev il.com" }},
		{"registry 注入", func(b map[string]any) { b["registry"] = "reg;rm -rf /" }},
		{"超长名称", func(b map[string]any) { b["repository_name"] = strings.Repeat("a", 201) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := validFavBody()
			tc.mutate(body)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, favAuthRequest(t, http.MethodPost, "/api/image-favorites", body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestImageFavoriteDelete(t *testing.T) {
	engine, _ := newFavTestEngine(t)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodPost, "/api/image-favorites", validFavBody()))
	var stored repository.ImageFavoriteRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodDelete, fmt.Sprintf("/api/image-favorites?id=%d", stored.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE code = %d, body = %s", rec.Code, rec.Body.String())
	}

	// 再删一次 → 404
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodDelete, fmt.Sprintf("/api/image-favorites?id=%d", stored.ID), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("重复 DELETE code = %d, want 404", rec.Code)
	}

	// 非法 id → 400
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodDelete, "/api/image-favorites?id=abc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 id code = %d, want 400", rec.Code)
	}
}

func TestImageFavoritesCap(t *testing.T) {
	engine, repo := newFavTestEngine(t)

	for i := 0; i < maxImageFavorites; i++ {
		rec := &repository.ImageFavoriteRecord{
			RepositoryName: fmt.Sprintf("lib/img%d", i),
			Tag:            "latest",
		}
		if _, err := repo.UpsertImageFavorite(rec); err != nil {
			t.Fatalf("seed upsert %d: %v", i, err)
		}
	}

	// 新增第 201 条 → 400
	body := validFavBody()
	body["repository_name"] = "library/new-one"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodPost, "/api/image-favorites", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("超限 POST code = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	// 更新已存在的收藏不受上限限制
	body = validFavBody()
	body["repository_name"] = "lib/img0"
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, favAuthRequest(t, http.MethodPost, "/api/image-favorites", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("更新已存在收藏 code = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestImageFavoritesUnauthenticated(t *testing.T) {
	engine, _ := newFavTestEngine(t)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/image-favorites"},
		{http.MethodPost, "/api/image-favorites"},
		{http.MethodDelete, "/api/image-favorites?id=1"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s code = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}
