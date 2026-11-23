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

func TestResolveSecretGeneratesRandomEachTime(t *testing.T) {
	first, err := resolveSecret("JWT_SECRET", "", true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := resolveSecret("JWT_SECRET", "", true)
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
	if first == second {
		t.Fatal("expected different secrets on consecutive calls")
	}
}

func TestResolveSecretUsesEnvValue(t *testing.T) {
	want := strings.Repeat("b", 32)

	got, err := resolveSecret("JWT_SECRET", "  "+want+"  ", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveSecretRejectsWeakEnvValue(t *testing.T) {
	if _, err := resolveSecret("JWT_SECRET", "change-me", true); err == nil {
		t.Fatal("expected error for weak env value")
	}
}

func TestResolveSecretDisallowGenerate(t *testing.T) {
	if _, err := resolveSecret("NODE_SECRET", "", false); err == nil {
		t.Fatal("expected error when generation is not allowed")
	}
}

func TestResolveSecretLogsGeneratedSecret(t *testing.T) {
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(original)

	got, err := resolveSecret("JWT_SECRET", "", true)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "JWT_SECRET") {
		t.Fatalf("expected log to mention variable name, got %q", out)
	}
	if !strings.Contains(out, got) {
		t.Fatalf("expected log to contain generated secret, got %q", out)
	}
}
