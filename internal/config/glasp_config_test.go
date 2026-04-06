package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGlaspConfigNotFoundReturnsDefault(t *testing.T) {
	root := t.TempDir()
	cfg, err := LoadGlaspConfig(root)
	if err != nil {
		t.Fatalf("LoadGlaspConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Archive.Pull {
		t.Fatal("expected archive.pull to be false by default")
	}
	if cfg.Archive.Push {
		t.Fatal("expected archive.push to be false by default")
	}
}

func TestSaveLoadGlaspConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	original := &GlaspConfig{Archive: ArchiveConfig{Pull: true, Push: true}}
	if err := SaveGlaspConfig(root, original); err != nil {
		t.Fatalf("SaveGlaspConfig failed: %v", err)
	}
	loaded, err := LoadGlaspConfig(root)
	if err != nil {
		t.Fatalf("LoadGlaspConfig failed: %v", err)
	}
	if loaded.Archive.Pull != original.Archive.Pull {
		t.Fatalf("archive.pull mismatch: expected %v, got %v", original.Archive.Pull, loaded.Archive.Pull)
	}
	if loaded.Archive.Push != original.Archive.Push {
		t.Fatalf("archive.push mismatch: expected %v, got %v", original.Archive.Push, loaded.Archive.Push)
	}
}

func TestEnsureGlaspDirCreatesClaspignore(t *testing.T) {
	root := t.TempDir()

	// No .claspignore exists yet — EnsureGlaspDir should create it with .glasp/ entry.
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("expected .claspignore to be created: %v", err)
	}
	if !strings.Contains(string(data), ".glasp/") {
		t.Errorf("expected .claspignore to contain '.glasp/', got: %q", string(data))
	}

	// Calling again should not duplicate the entry.
	// Remove .glasp dir to trigger the "created" path again.
	os.RemoveAll(filepath.Join(root, ".glasp"))
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir (2nd call) failed: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("failed to read .claspignore: %v", err)
	}
	if strings.Count(string(data), ".glasp/") != 1 {
		t.Errorf("expected exactly one '.glasp/' entry, got: %q", string(data))
	}
}

func TestEnsureGlaspDirExistingClaspignore(t *testing.T) {
	root := t.TempDir()

	// Pre-existing .claspignore without trailing newline.
	if err := os.WriteFile(filepath.Join(root, ".claspignore"), []byte("node_modules/"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("failed to read .claspignore: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, ".glasp/") {
		t.Errorf("expected .glasp/ to be appended, got: %q", content)
	}
	if !strings.Contains(content, "node_modules/") {
		t.Errorf("expected original content to be preserved, got: %q", content)
	}
	// Should have a newline between original content and new entry.
	if strings.Contains(content, "node_modules/.glasp/") {
		t.Errorf("expected newline separator, got: %q", content)
	}
}

func TestEnsureGlaspDirAlreadyHasEntry(t *testing.T) {
	root := t.TempDir()

	// .claspignore already contains .glasp/
	if err := os.WriteFile(filepath.Join(root, ".claspignore"), []byte(".glasp/\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("failed to read .claspignore: %v", err)
	}
	if strings.Count(string(data), ".glasp/") != 1 {
		t.Errorf("expected no duplicate, got: %q", string(data))
	}
}

func TestLoadGlaspConfigInvalidJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, glaspConfigDirName), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, glaspConfigDirName, glaspConfigFileName), []byte("{"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := LoadGlaspConfig(root); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
