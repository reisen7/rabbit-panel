package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyEnvLines_Basic(t *testing.T) {
	os.Unsetenv("RABBIT_TEST_BASIC")
	os.Unsetenv("RABBIT_TEST_NUM")

	n := applyEnvLines("RABBIT_TEST_BASIC=hello\nRABBIT_TEST_NUM=3958\n")
	if n != 2 {
		t.Fatalf("expected 2 loaded, got %d", n)
	}
	if got := os.Getenv("RABBIT_TEST_BASIC"); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := os.Getenv("RABBIT_TEST_NUM"); got != "3958" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyEnvLines_NeverOverridesExisting(t *testing.T) {
	t.Setenv("RABBIT_TEST_KEEP", "real-env")

	n := applyEnvLines("RABBIT_TEST_KEEP=from-file\n")
	if n != 0 {
		t.Fatalf("expected 0 loaded, got %d", n)
	}
	if got := os.Getenv("RABBIT_TEST_KEEP"); got != "real-env" {
		t.Fatalf("real env overwritten: %q", got)
	}
}

func TestApplyEnvLines_SkipsCommentsBlanksGarbage(t *testing.T) {
	os.Unsetenv("RABBIT_TEST_VALID")
	content := `# 注释行

这是垃圾行没有等号
=没有键
9BAD_KEY=x
RABBIT_TEST_VALID=ok
`
	n := applyEnvLines(content)
	if n != 1 {
		t.Fatalf("expected 1 loaded, got %d", n)
	}
	if got := os.Getenv("RABBIT_TEST_VALID"); got != "ok" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyEnvLines_EmptyEnvTreatedAsUnset(t *testing.T) {
	// compose 的 ${VAR:-} 展开会产生存在但为空的变量，文件值应当生效
	t.Setenv("RABBIT_TEST_EMPTY", "")

	n := applyEnvLines("RABBIT_TEST_EMPTY=from-file\n")
	if n != 1 {
		t.Fatalf("expected 1 loaded, got %d", n)
	}
	if got := os.Getenv("RABBIT_TEST_EMPTY"); got != "from-file" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyEnvLines_ExportPrefixAndQuotes(t *testing.T) {
	os.Unsetenv("RABBIT_TEST_EXPORT")
	os.Unsetenv("RABBIT_TEST_DQ")
	os.Unsetenv("RABBIT_TEST_SQ")

	n := applyEnvLines("export RABBIT_TEST_EXPORT=a\nRABBIT_TEST_DQ=\"b c\"\nRABBIT_TEST_SQ='d'\n")
	if n != 3 {
		t.Fatalf("expected 3 loaded, got %d", n)
	}
	if got := os.Getenv("RABBIT_TEST_EXPORT"); got != "a" {
		t.Fatalf("export prefix: got %q", got)
	}
	if got := os.Getenv("RABBIT_TEST_DQ"); got != "b c" {
		t.Fatalf("double quotes: got %q", got)
	}
	if got := os.Getenv("RABBIT_TEST_SQ"); got != "d" {
		t.Fatalf("single quotes: got %q", got)
	}
}

func TestLoadEnvFile_ReadsDataSecretsFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0755)
	os.WriteFile(filepath.Join(dir, "data", "secrets.env"), []byte("RABBIT_TEST_FILE=fromdata\n"), 0600)

	old := envFilePath
	envFilePath = filepath.Join(dir, "data", "secrets.env")
	defer func() { envFilePath = old }()

	os.Unsetenv("RABBIT_TEST_FILE")
	loadEnvFile()
	if got := os.Getenv("RABBIT_TEST_FILE"); got != "fromdata" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadEnvFile_MissingFileSilent(t *testing.T) {
	old := envFilePath
	envFilePath = filepath.Join(t.TempDir(), "nonexistent.env")
	defer func() { envFilePath = old }()

	loadEnvFile() // 不 panic 即通过
}
