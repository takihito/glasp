package main

import (
	"os"
	"path/filepath"
	"testing"

	"glasp/internal/config"
)

func TestConfigInitCreatesConfig(t *testing.T) {
	root := useTempDir(t)
	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("ConfigInitCmd.Run failed: %v", err)
	}

	cfgPath := filepath.Join(root, ".glasp", "config.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config.json to exist: %v", err)
	}
	cfg, err := config.LoadGlaspConfig(root)
	if err != nil {
		t.Fatalf("LoadGlaspConfig failed: %v", err)
	}
	if cfg.Archive.Pull {
		t.Fatalf("expected archive.pull to be false by default")
	}
	if cfg.Archive.Push {
		t.Fatalf("expected archive.push to be false by default")
	}
}

func TestConfigInitRejectsExistingConfig(t *testing.T) {
	root := useTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, ".glasp"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".glasp", "config.json"), []byte(`{"archive":{"pull":true}}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err == nil {
		t.Fatalf("expected error when config already exists")
	}
}

func TestConfigInitCreatesConfigWithoutClaspConfig(t *testing.T) {
	root := useTempDir(t)
	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("expected config init to work without .clasp.json in %s: %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".glasp", "config.json")); err != nil {
		t.Fatalf("expected config.json to exist: %v", err)
	}
}
