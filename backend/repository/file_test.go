package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateComposeProjectName(t *testing.T) {
	valid := []string{"myproject", "my-project", "my_project", "proj.ect", "Proj123"}
	for _, name := range valid {
		if err := ValidateComposeProjectName(name); err != nil {
			t.Errorf("expected %q to be valid, got %v", name, err)
		}
	}

	invalid := []string{
		"", " ", ".", "..",
		"../etc", `..\etc`, "foo/../bar", "foo/bar", `foo\bar`,
		"/etc", "my..project", " leading", "trailing ",
	}
	for _, name := range invalid {
		if err := ValidateComposeProjectName(name); err == nil {
			t.Errorf("expected %q to be invalid, got nil", name)
		}
	}
}

func TestComposeProjectPathTraversalBlocked(t *testing.T) {
	repo := NewFileRepository()
	base := t.TempDir()
	repo.composeBaseDir = filepath.Join(base, "compose_projects")

	// 在基目录外放一个诱饵目录，穿越成功时它会被删掉
	decoy := filepath.Join(base, "decoy")
	if err := os.MkdirAll(decoy, 0755); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteComposeProject("../decoy"); err == nil {
		t.Fatal("expected traversal delete to be rejected")
	}
	if _, err := os.Stat(decoy); err != nil {
		t.Fatal("decoy directory must survive traversal attempt")
	}

	if _, err := repo.GetComposeFile("../../etc/passwd"); err == nil {
		t.Fatal("expected traversal read to be rejected")
	}
	if err := repo.SaveComposeFile("../evil", "x"); err == nil {
		t.Fatal("expected traversal write to be rejected")
	}
	if err := repo.CreateComposeProject("../evil", "x"); err == nil {
		t.Fatal("expected traversal create to be rejected")
	}
	if repo.ComposeProjectExists("../compose_projects") {
		t.Fatal("expected traversal exists-check to be false")
	}
}
