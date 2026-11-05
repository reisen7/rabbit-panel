package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDockerHostFromContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_HOST", "")

	if got := dockerHostFromContext(); got != "" {
		t.Fatalf("empty config dir host = %q", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"desktop-linux"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	metaDir := filepath.Join(dir, "contexts", "meta", "abc")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"Name":"desktop-linux","Endpoints":{"docker":{"Host":"npipe:////./pipe/dockerDesktopLinuxEngine","SkipTLSVerify":false}}}`
	if err := os.WriteFile(filepath.Join(metaDir, "meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	got := dockerHostFromContext()
	if got != "npipe:////./pipe/dockerDesktopLinuxEngine" {
		t.Fatalf("host = %q", got)
	}

	t.Setenv("DOCKER_HOST", "npipe:////./pipe/docker_engine")
	if got := dockerHostFromContext(); got != "" {
		t.Fatalf("DOCKER_HOST set should defer to FromEnv, got %q", got)
	}
}

func TestDockerHostIgnoresDefaultContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_HOST", "")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"default"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dockerHostFromContext(); got != "" {
		t.Fatalf("default context host = %q", got)
	}
}
