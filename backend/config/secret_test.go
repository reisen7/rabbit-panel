package config

import (
	"bytes"
	"encoding/hex"
	"log"
	"strings"
	"testing"
)

func TestRequireUserSecret(t *testing.T) {
	rejects := []string{
		"",
		"   ",
		"change-me",
		"node-secret",
		"change-me-in-production",
		"rabbit-panel-secret-key-change-in-production",
		"rabbit-panel-node-secret-change-in-production",
		"short-secret",
	}
	for _, value := range rejects {
		if _, err := requireUserSecret("JWT_SECRET", value); err == nil {
			t.Fatalf("expected reject for %q", value)
		}
	}

	want := strings.Repeat("a", 32)
	got, err := requireUserSecret("NODE_SECRET", "  "+want+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q", got)
	}
}

// 新行为：密钥生成后持久化——同一数据目录下重复解析必须返回相同值（重启稳定），
// 不同数据目录各自生成不同值。
func TestResolveSecretPersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	first, err := resolveSecretWithDir("JWT_SECRET", "", true, dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := resolveSecretWithDir("JWT_SECRET", "", true, dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, got := range []string{first, second} {
		if len(got) != 64 {
			t.Fatalf("expected 64-char hex secret, got %d chars", len(got))
		}
		if _, err := hex.DecodeString(got); err != nil {
			t.Fatalf("secret is not valid hex: %v", err)
		}
	}
	if first != second {
		t.Fatal("expected persisted secret to be stable across calls")
	}

	other, err := resolveSecretWithDir("JWT_SECRET", "", true, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("different data dirs must generate independent secrets")
	}
}

// 两个密钥共用同一持久化文件，互不覆盖。
func TestPersistedSecretsShareFile(t *testing.T) {
	dir := t.TempDir()
	jwt, err := resolveSecretWithDir("JWT_SECRET", "", true, dir)
	if err != nil {
		t.Fatal(err)
	}
	node, err := resolveSecretWithDir("NODE_SECRET", "", true, dir)
	if err != nil {
		t.Fatal(err)
	}
	if jwt == node {
		t.Fatal("jwt and node secrets must differ")
	}
	// 重读验证都在文件里
	if got := readPersistedSecret(dir, "JWT_SECRET"); got != jwt {
		t.Fatalf("jwt not persisted: %q", got)
	}
	if got := readPersistedSecret(dir, "NODE_SECRET"); got != node {
		t.Fatalf("node not persisted: %q", got)
	}
}

func TestResolveSecretUsesEnvValue(t *testing.T) {
	want := strings.Repeat("b", 32)

	got, err := resolveSecretWithDir("JWT_SECRET", "  "+want+"  ", true, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveSecretRejectsWeakEnvValue(t *testing.T) {
	if _, err := resolveSecretWithDir("JWT_SECRET", "change-me", true, t.TempDir()); err == nil {
		t.Fatal("expected error for weak env value")
	}
}

func TestResolveSecretDisallowGenerate(t *testing.T) {
	if _, err := resolveSecretWithDir("NODE_SECRET", "", false, t.TempDir()); err == nil {
		t.Fatal("expected error when generation is not allowed")
	}
}

func TestResolveSecretLogsLocationNotSecret(t *testing.T) {
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(original)

	dir := t.TempDir()
	got, err := resolveSecretWithDir("JWT_SECRET", "", true, dir)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "JWT_SECRET") {
		t.Fatalf("expected log to mention variable name, got %q", out)
	}
	// 密钥本身绝不能出现在日志里（持久化后查看文件即可，无需打日志）
	if strings.Contains(out, got) {
		t.Fatal("secret value must not be printed to log")
	}
	if !strings.Contains(out, "secrets.env") {
		t.Fatalf("expected log to mention storage location, got %q", out)
	}
}
