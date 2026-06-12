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
func TestPullCommandFlow(t *testing.T) {
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

	if err := (&PullCmd{}).Run(nil); err != nil {
		t.Fatalf("PullCmd.Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "Code.js")); err != nil {
		t.Fatalf("expected Code.js to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "appsscript.json")); err != nil {
		t.Fatalf("expected appsscript.json to exist: %v", err)
	}
}

func TestPullCommandPassesAuthPath(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	fake := &fakeScriptClient{getContentResp: sampleContent()}
	var gotAuthPath string
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		gotAuthPath = authPath
		return fake, nil
	}

	if err := (&PullCmd{Auth: " ./auth/.clasprc.json "}).Run(nil); err != nil {
		t.Fatalf("PullCmd.Run failed: %v", err)
	}
	if gotAuthPath != filepath.Clean("./auth/.clasprc.json") {
		t.Fatalf("expected cleaned auth path, got %q", gotAuthPath)
	}
}

func TestPullCommandRejectsEmptyAuthPath(t *testing.T) {
	useTempDir(t)
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	err := (&PullCmd{Auth: "   "}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for empty --auth path")
	}
	if !strings.Contains(err.Error(), "--auth path is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPullCommandDryRunSkipsAPIAndWrites(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected dryrun pull to skip client creation")
		return nil, nil
	}

	if err := (&PullCmd{DryRun: true}).Run(nil); err != nil {
		t.Fatalf("PullCmd.Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "Code.js")); !os.IsNotExist(err) {
		t.Fatalf("expected no pulled files to be written during dryrun")
	}
}

func TestPullCommandDryRunRejectsEmptyAuthPath(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	err := (&PullCmd{DryRun: true, Auth: "   "}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for empty --auth path in dryrun pull")
	}
	if !strings.Contains(err.Error(), "--auth path is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPullCommandRejectsTSXFileExtension(t *testing.T) {
	root := useTempDir(t)
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		RootDir:  "src",
		Extra: map[string]json.RawMessage{
			"fileExtension": json.RawMessage(`"tsx"`),
		},
	}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	err := (&PullCmd{}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for tsx fileExtension")
	}
	if !strings.Contains(err.Error(), `fileExtension "tsx" is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
