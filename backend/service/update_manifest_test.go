package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testManifestBody = `{"version":"9.9.9","release_url":"https://example.com/release"}`

func newManifestTestService() *UpdateService {
	return NewUpdateService(nil, BuildInfo{Version: "1.0.0"})
}

func newManifestServer(t *testing.T, delay time.Duration, hits *atomic.Int32, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if delay > 0 {
			time.Sleep(delay)
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			fmt.Fprintln(w, "boom")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, testManifestBody)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchManifestRacesFallback(t *testing.T) {
	var primaryHits, fallbackHits atomic.Int32
	primary := newManifestServer(t, 3*time.Second, &primaryHits, http.StatusOK)
	fallback := newManifestServer(t, 0, &fallbackHits, http.StatusOK)

	t.Setenv("RABBIT_UPDATE_MANIFEST_URL", primary.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", fallback.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_MIRROR_URL", "off")

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
		t.Fatalf("fetch took %v, expected the fast fallback to win", elapsed)
	}
}

func TestGetManifestCachesSuccess(t *testing.T) {
	var hits atomic.Int32
	srv := newManifestServer(t, 0, &hits, http.StatusOK)
	t.Setenv("RABBIT_UPDATE_MANIFEST_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_MIRROR_URL", "off")

	svc := newManifestTestService()
	if _, err := svc.getManifest(context.Background(), false); err != nil {
		t.Fatalf("first getManifest: %v", err)
	}
	if _, err := svc.getManifest(context.Background(), false); err != nil {
		t.Fatalf("second getManifest: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server hit %d times, expected 1 (cache)", got)
	}
}

func TestGetManifestForceBypassesCache(t *testing.T) {
	var hits atomic.Int32
	srv := newManifestServer(t, 0, &hits, http.StatusOK)
	t.Setenv("RABBIT_UPDATE_MANIFEST_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_MIRROR_URL", "off")

	svc := newManifestTestService()
	if _, err := svc.getManifest(context.Background(), false); err != nil {
		t.Fatalf("first getManifest: %v", err)
	}
	if _, err := svc.getManifest(context.Background(), true); err != nil {
		t.Fatalf("forced getManifest: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("server hit %d times, expected 2 (force refetch)", got)
	}
}

func TestGetManifestCachesFailure(t *testing.T) {
	var hits atomic.Int32
	srv := newManifestServer(t, 0, &hits, http.StatusInternalServerError)
	t.Setenv("RABBIT_UPDATE_MANIFEST_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_MIRROR_URL", "off")

	svc := newManifestTestService()
	if _, err := svc.getManifest(context.Background(), false); err == nil {
		t.Fatal("expected error from failing server")
	}
	if _, err := svc.getManifest(context.Background(), false); err == nil {
		t.Fatal("expected cached error within failure TTL")
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server hit %d times, expected 1 (failure cache)", got)
	}
}

func TestGetManifestConcurrentSingleFetch(t *testing.T) {
	var hits atomic.Int32
	srv := newManifestServer(t, 50*time.Millisecond, &hits, http.StatusOK)
	t.Setenv("RABBIT_UPDATE_MANIFEST_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", srv.URL)
	t.Setenv("RABBIT_UPDATE_MANIFEST_MIRROR_URL", "off")

	svc := newManifestTestService()
	const callers = 5
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.getManifest(context.Background(), false)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	// 并发共享同一次抓取；首个成功即取消的竞争下主+备合计命中数 ≤2。
	if got := hits.Load(); got > 2 {
		t.Fatalf("server hit %d times, expected <=2 for %d concurrent callers", got, callers)
	}
}
