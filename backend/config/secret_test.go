package config

import (
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
