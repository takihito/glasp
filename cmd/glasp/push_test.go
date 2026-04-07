package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"
	"google.golang.org/api/script/v1"
)

func TestPushCommandArchiveFlag(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Archive: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
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
	if _, err := os.Stat(filepath.Join(archiveDir, "working", "src", "Code.gs")); err != nil {
		t.Fatalf("expected working Code.gs to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "payload", "src", "Code.gs")); err != nil {
		t.Fatalf("expected payload Code.gs to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "canonical")); !os.IsNotExist(err) {
		t.Fatalf("expected canonical directory to be absent on push")
	}
}

func TestPushCommandArchiveConfig(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := config.SaveGlaspConfig(root, &config.GlaspConfig{Archive: config.ArchiveConfig{Push: true}}); err != nil {
		t.Fatalf("SaveGlaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Archive: false}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
}

func TestPushCommandArchiveWithTypeScriptConversion(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.ts"), []byte("const msg: string = 'hi'"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Archive: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	file := findContentFile(fake.updateContent, "Code")
	if file == nil {
		t.Fatalf("expected pushed Code file")
	}
	if strings.Contains(file.Source, ": string") {
		t.Fatalf("expected TypeScript annotation to be removed from payload, got: %s", file.Source)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
	archiveDir := filepath.Join(archiveBase, entries[0].Name())
	if _, err := os.Stat(filepath.Join(archiveDir, "working", "src", "Code.ts")); err != nil {
		t.Fatalf("expected working Code.ts to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "payload", "src", "Code.js")); err != nil {
		t.Fatalf("expected payload Code.js to exist: %v", err)
	}
}

func TestPushCommandAutoTranspileTSWithoutFileExtension(t *testing.T) {
	root := useTempDir(t)
	// No fileExtension set in .clasp.json
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		RootDir:  "src",
	}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.ts"), []byte("const msg: string = 'hi'"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Util.js"), []byte("function util() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}

	// .ts file should be transpiled (type annotation removed).
	codeFile := findContentFile(fake.updateContent, "Code")
	if codeFile == nil {
		t.Fatalf("expected pushed Code file")
	}
	if strings.Contains(codeFile.Source, ": string") {
		t.Fatalf("expected TypeScript annotation to be removed, got: %s", codeFile.Source)
	}

	// .js file should be passed through unchanged.
	utilFile := findContentFile(fake.updateContent, "Util")
	if utilFile == nil {
		t.Fatalf("expected pushed Util file")
	}
	if utilFile.Source != "function util() {}" {
		t.Fatalf("expected .js file to be unchanged, got: %s", utilFile.Source)
	}
}

func TestArchivePushRunWritesFailedManifestOnError(t *testing.T) {
	root := useTempDir(t)
	working := []syncer.ProjectFile{
		{
			LocalPath: "src/Code.gs",
			Type:      "SERVER_JS",
			Source:    "function a() {}",
		},
	}
	payload := []syncer.ProjectFile{
		{
			LocalPath: "../evil.gs",
			Type:      "SERVER_JS",
			Source:    "function evil() {}",
		},
	}

	_, err := archivePushRun(root, "script-id", working, payload, "gs", "")
	if err == nil {
		t.Fatalf("expected archivePushRun to fail")
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
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
}
