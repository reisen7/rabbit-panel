package router

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteEmbeddedCacheAndGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.Repeat("rabbit-panel-static-asset;", 80)

	cases := []struct {
		name       string
		path       string
		gzip       bool
		wantCache  string
		wantGzip   bool
	}{
		{
			name:      "带哈希的脚本长期缓存并压缩",
			path:      "assets/element-plus-CMtISwqQ.js",
			gzip:      true,
			wantCache: "public, max-age=31536000, immutable",
			wantGzip:  true,
		},
		{
			name:      "入口页不长期缓存",
			path:      "index.html",
			gzip:      true,
			wantCache: "no-cache",
			wantGzip:  true,
		},
		{
			name:      "客户端不接受 gzip 时原样返回",
			path:      "assets/index-CHqFh1vD.css",
			gzip:      false,
			wantCache: "public, max-age=31536000, immutable",
			wantGzip:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/"+tc.path, nil)
			if tc.gzip {
				c.Request.Header.Set("Accept-Encoding", "gzip, deflate, br")
			}

			writeEmbedded(c, tc.path, []byte(body))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d", w.Code)
			}
			if got := w.Header().Get("Cache-Control"); got != tc.wantCache {
				t.Fatalf("Cache-Control = %q, want %q", got, tc.wantCache)
			}
			if tc.wantGzip {
				if w.Header().Get("Content-Encoding") != "gzip" {
					t.Fatal("expected gzip content encoding")
				}
				if w.Body.Len() >= len(body) {
					t.Fatalf("gzip did not shrink body: %d >= %d", w.Body.Len(), len(body))
				}
				zr, err := gzip.NewReader(w.Body)
				if err != nil {
					t.Fatal(err)
				}
				defer zr.Close()
				got, err := io.ReadAll(zr)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != body {
					t.Fatalf("decoded body mismatch")
				}
				return
			}
			if w.Header().Get("Content-Encoding") != "" {
				t.Fatalf("unexpected content encoding %q", w.Header().Get("Content-Encoding"))
			}
			if w.Body.String() != body {
				t.Fatal("raw body mismatch")
			}
		})
	}
}

func TestWriteEmbeddedSkipsGzipForImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/assets/logo-abc.png", nil)
	c.Request.Header.Set("Accept-Encoding", "gzip")
	payload := bytesRepeat(2048)

	writeEmbedded(c, "assets/logo-abc.png", payload)

	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("png should not be gzipped, got %q", w.Header().Get("Content-Encoding"))
	}
	if w.Body.Len() != len(payload) {
		t.Fatalf("body len = %d, want %d", w.Body.Len(), len(payload))
	}
}

func bytesRepeat(n int) []byte {
	return []byte(strings.Repeat("x", n))
}
