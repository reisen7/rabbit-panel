package service

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeShowdocToken(t *testing.T) {
	token, keep, err := normalizeShowdocToken("  https://push.showdoc.com.cn/server/api/push/Abcd1234ef  ")
	if err != nil || keep || token != "Abcd1234ef" {
		t.Fatalf("extract token: %q keep=%v err=%v", token, keep, err)
	}
	if _, keep, err = normalizeShowdocToken(""); err != nil || !keep {
		t.Fatalf("empty should keep the saved token, keep=%v err=%v", keep, err)
	}
	if _, keep, err = normalizeShowdocToken("ab12****ef34"); err != nil || !keep {
		t.Fatalf("mask should keep the saved token, keep=%v err=%v", keep, err)
	}
	if _, _, err = normalizeShowdocToken("https://example.com/hook"); err == nil {
		t.Fatal("a non-showdoc address should be rejected")
	}
}

func TestNotifySaveKeepsTokenAndFileMode(t *testing.T) {
	dir := t.TempDir()
	svc := NewNotifyService()
	svc.path = filepath.Join(dir, "notify.json")
	svc.config = defaultNotifyConfig()
	svc.config.Token = "Abcd1234ef"

	saved, err := svc.Save(NotifyConfig{
		Enabled:         true,
		Token:           "",
		AllContainers:   false,
		Containers:      []string{"web", "web"},
		ExitNonZero:     true,
		CooldownMinutes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !saved.HasToken || saved.Token != "" || saved.TokenMask != "Abcd****34ef" {
		t.Fatalf("public settings = %+v", saved)
	}
	if len(saved.Containers) != 1 || saved.Containers[0] != "web" {
		t.Fatalf("containers = %#v", saved.Containers)
	}
	info, err := os.Stat(svc.path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %o", info.Mode().Perm())
	}
}
