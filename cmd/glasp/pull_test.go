package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takihito/glasp/internal/config"
)

func TestPullCommandArchiveFlag(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	fake := &fakeScriptClient{getContentResp: sampleContent()}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PullCmd{Archive: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PullCmd.Run failed: %v", err)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "pull")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
	if !entries[0].IsDir() {
		t.Fatalf("expected archive entry to be a directory")
	}
	archiveDir := filepath.Join(archiveBase, entries[0].Name())
	if _, err := os.Stat(filepath.Join(archiveDir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json to exist: %v", err)
	}
	manifestData, err := os.ReadFile(filepath.Join(archiveDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	var manifest archiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}
	if manifest.Status != "success" {
		t.Fatalf("expected manifest status success, got %s", manifest.Status)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "canonical", "src", "Code.js")); err != nil {
		t.Fatalf("expected canonical Code.js to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "working", "src", "Code.js")); err != nil {
		t.Fatalf("expected working Code.js to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "payload")); !os.IsNotExist(err) {
		t.Fatalf("expected payload directory to be absent on pull")
	}

	if _, err := os.Stat(filepath.Join(root, "src", "Code.js")); err != nil {
		t.Fatalf("expected Code.js to exist: %v", err)
	}
}

func TestPullCommandArchiveConfig(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := config.SaveGlaspConfig(root, &config.GlaspConfig{Archive: config.ArchiveConfig{Pull: true}}); err != nil {
		t.Fatalf("SaveGlaspConfig failed: %v", err)
	}

	fake := &fakeScriptClient{getContentResp: sampleContent()}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PullCmd{Archive: false}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PullCmd.Run failed: %v", err)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "pull")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
}

func TestPullCommandArchiveWithTypeScriptConversion(t *testing.T) {
	root := useTempDir(t)
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		RootDir:  "src",
		Extra: map[string]json.RawMessage{
			"fileExtension": json.RawMessage(`"ts"`),
		},
	}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	fake := &fakeScriptClient{getContentResp: sampleContent()}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PullCmd{Archive: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PullCmd.Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "src", "Code.ts")); err != nil {
		t.Fatalf("expected converted Code.ts to exist: %v", err)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "pull")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
	archiveDir := filepath.Join(archiveBase, entries[0].Name())
	if _, err := os.Stat(filepath.Join(archiveDir, "canonical", "src", "Code.js")); err != nil {
		t.Fatalf("expected canonical Code.js to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "working", "src", "Code.ts")); err != nil {
		t.Fatalf("expected working Code.ts to exist: %v", err)
	}
}

func TestArchivePullRunWritesFailedManifestOnError(t *testing.T) {
	root := useTempDir(t)
	cfg := &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	_, err := archivePullRun(root, "script-id", cfg, sampleContent(), nil, "ts", "")
	if err == nil {
		t.Fatalf("expected archivePullRun to fail")
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "pull")
	entries, readErr := os.ReadDir(archiveBase)
	if readErr != nil {
		t.Fatalf("expected archive dir to exist: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
	manifestData, readErr := os.ReadFile(filepath.Join(archiveBase, entries[0].Name(), "manifest.json"))
	if readErr != nil {
		t.Fatalf("expected failed manifest to exist: %v", readErr)
	}
	var manifest archiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}
	if manifest.Status != "failed" {
		t.Fatalf("expected manifest status failed, got %s", manifest.Status)
	}
	if !strings.Contains(err.Error(), "content is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}
