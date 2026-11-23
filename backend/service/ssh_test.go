package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSshServiceEncryptDecryptRoundtrip(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewSshService(nil, dir)
	if err != nil {
		t.Fatalf("NewSshService failed: %v", err)
	}

	enc, err := svc.encrypt("s3cret-password")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if enc == "s3cret-password" {
		t.Fatal("ciphertext equals plaintext")
	}
	plain, err := svc.decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if plain != "s3cret-password" {
		t.Fatalf("roundtrip mismatch: got %q", plain)
	}
}

func TestSshServiceKeyPersistedAcrossRestarts(t *testing.T) {
	dir := t.TempDir()

	svc1, err := NewSshService(nil, dir)
	if err != nil {
		t.Fatalf("first NewSshService failed: %v", err)
	}
	enc, err := svc1.encrypt("pw")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// 模拟重启：新实例从同一 data 目录加载密钥，应能解开旧密文
	svc2, err := NewSshService(nil, dir)
	if err != nil {
		t.Fatalf("second NewSshService failed: %v", err)
	}
	plain, err := svc2.decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt after restart failed: %v", err)
	}
	if plain != "pw" {
		t.Fatalf("roundtrip after restart mismatch: got %q", plain)
	}
}

func TestSshServiceKeyFilePermissions(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewSshService(nil, dir); err != nil {
		t.Fatalf("NewSshService failed: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, ".ssh_secret_key"))
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}
	if info.IsDir() {
		t.Fatal("key path is a directory")
	}
}

func TestSshServiceDecryptTampered(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewSshService(nil, dir)
	if err != nil {
		t.Fatalf("NewSshService failed: %v", err)
	}
	if _, err := svc.decrypt("not-base64!!!"); err == nil {
		t.Fatal("expected error for invalid ciphertext")
	}
}
