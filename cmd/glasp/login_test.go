package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/takihito/glasp/internal/config"
)

// writeAuthFixture writes a minimal .clasprc.json with dummy token data.
func writeAuthFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, ".clasprc.json")
	payload := `{"token":{"access_token":"dummy-access-token","refresh_token":"dummy-refresh-token","token_type":"Bearer"}}`
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatalf("write auth fixture failed: %v", err)
	}
	return path
}

func TestLoginCommandWithAuthUsesParentProjectRoot(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	subdir := filepath.Join(root, "src")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	authPath := writeAuthFixture(t, root)

	cmd := LoginCmd{Auth: authPath}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("LoginCmd.Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".glasp", "access.json")); err != nil {
		t.Fatalf("expected token cache under project root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(subdir, ".clasp.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no .clasp.json created in subdirectory, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(subdir, ".glasp")); !os.IsNotExist(err) {
		t.Fatalf("expected no .glasp directory created in subdirectory, stat err=%v", err)
	}
}

func TestLoginCommandWithAuthCreatesConfigWhenNoProject(t *testing.T) {
	root := useTempDir(t)
	authPath := writeAuthFixture(t, root)

	cmd := LoginCmd{Auth: authPath}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("LoginCmd.Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".clasp.json")); err != nil {
		t.Fatalf("expected .clasp.json created in current directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".glasp", "access.json")); err != nil {
		t.Fatalf("expected token cache under current directory: %v", err)
	}
}
