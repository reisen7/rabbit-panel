package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchManifestMirrorWins(t *testing.T) {
	var mirrorHits atomic.Int32
	// 主/备用都返回 500，mirror 立即成功
	badHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	primary := httptest.NewServer(badHandler)
	fallback := httptest.NewServer(badHandler)
	mirror := newManifestServer(t, 0, &mirrorHits, http.StatusOK)
	t.Cleanup(primary.Close)
	t.Cleanup(fallback.Close)

	t.Setenv("RABBIT_UPDATE_MANIFEST_URL", primary.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", fallback.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_MIRROR_URL", mirror.URL)

	svc := newManifestTestService()
	start := time.Now()
	manifest, err := svc.fetchManifest(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("fetchManifest: %v", err)
	}
	if manifest.Version != "9.9.9" {
		t.Fatalf("unexpected version %q", manifest.Version)
	}
	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("fetch took %v, expected the mirror to win", elapsed)
	}
	if mirrorHits.Load() != 1 {
		t.Fatalf("mirror hit %d times, expected 1", mirrorHits.Load())
	}
}

func TestFetchManifestMirrorDisabled(t *testing.T) {
	var mirrorHits atomic.Int32
	src := newManifestServer(t, 0, &atomic.Int32{}, http.StatusOK)
	newManifestServer(t, 0, &mirrorHits, http.StatusOK)
	t.Setenv("RABBIT_UPDATE_MANIFEST_URL", src.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", src.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_MIRROR_URL", "OFF")

	svc := newManifestTestService()
	if _, err := svc.fetchManifest(context.Background()); err != nil {
		t.Fatalf("fetchManifest: %v", err)
	}
	if mirrorHits.Load() != 0 {
		t.Fatalf("mirror was requested %d times despite off", mirrorHits.Load())
	}
}

func TestApplyDownloadProxy(t *testing.T) {
	gh := "https://github.com/reisen7/rabbit-panel/releases/download/v1/x"
	cases := []struct {
		name  string
		raw   string
		proxy string
		want  string
	}{
		{"empty proxy", gh, "", gh},
		{"whitespace proxy", gh, "   ", gh},
		{"invalid scheme", gh, "ftp://x/", gh},
		{"missing scheme", gh, "x.com/", gh},
		{"github with trailing slash", gh, "https://ghfast.top/", "https://ghfast.top/" + gh},
		{"github without trailing slash", gh, "https://ghfast.top", "https://ghfast.top/" + gh},
		{"github extra slashes", gh, "https://ghfast.top//", "https://ghfast.top/" + gh},
		{"non-github host unchanged", "https://example.com/x", "https://ghfast.top/", "https://example.com/x"},
		{"github subdomain unchanged", "https://objects.githubusercontent.com/x", "https://ghfast.top/", "https://objects.githubusercontent.com/x"},
		{"garbage raw url", "://bad", "https://ghfast.top/", "://bad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := applyDownloadProxy(tc.raw, tc.proxy); got != tc.want {
				t.Fatalf("applyDownloadProxy(%q, %q) = %q, want %q", tc.raw, tc.proxy, got, tc.want)
			}
		})
	}
}

func TestRequireManifestSHA(t *testing.T) {
	valid := strings.Repeat("a1", 32)
	manifest := &UpdateManifest{SHA256: map[string]string{"linux-amd64": strings.ToUpper(valid)}}
	sha, err := requireManifestSHA(manifest, "linux-amd64")
	if err != nil {
		t.Fatalf("valid sha rejected: %v", err)
	}
	if sha != valid {
		t.Fatalf("expected lowercase sha, got %q", sha)
	}

	bad := []struct {
		name string
		sha  map[string]string
		key  string
	}{
		{"missing", map[string]string{}, "linux-amd64"},
		{"nil map", nil, "linux-amd64"},
		{"short", map[string]string{"linux-amd64": "abc"}, "linux-amd64"},
		{"non-hex", map[string]string{"linux-amd64": strings.Repeat("zz", 32)}, "linux-amd64"},
		{"wrong arch key", map[string]string{"windows-amd64": valid}, "linux-amd64"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			m := &UpdateManifest{SHA256: tc.sha}
			_, err := requireManifestSHA(m, tc.key)
			if err == nil || !strings.Contains(err.Error(), "refusing to update") {
				t.Fatalf("expected refusing-to-update error, got %v", err)
			}
		})
	}
}
