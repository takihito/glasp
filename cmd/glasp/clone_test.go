package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"google.golang.org/api/script/v1"
)

func TestCloneCommandFlow(t *testing.T) {
	root := useTempDir(t)
	fake := &fakeScriptClient{
		getProjectResp: &script.Project{ParentId: "parent-id"},
		getContentResp: sampleContent(),
	}

	var cacheFile string
	var gotAuthPath string
	origWithCache := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCache })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		cacheFile = cachePath
		gotAuthPath = authPath
		return fake, nil
	}

	cmd := CloneCmd{ScriptID: "script-id"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CloneCmd.Run failed: %v", err)
	}
	expectedCache := filepath.Join(root, ".glasp", "access.json")
	if cacheFile != expectedCache {
		t.Fatalf("expected cache path %s, got %s", expectedCache, cacheFile)
	}
	if gotAuthPath != "" {
		t.Fatalf("expected empty auth path, got %q", gotAuthPath)
	}

	cfg, err := config.LoadClaspConfig(root)
	if err != nil {
		t.Fatalf("LoadClaspConfig failed: %v", err)
	}
	if cfg.ScriptID != "script-id" {
		t.Fatalf("expected ScriptID script-id, got %s", cfg.ScriptID)
	}
	if cfg.RootDir != "./" {
		t.Fatalf("expected RootDir ./, got %s", cfg.RootDir)
	}
	if cfg.Extra == nil {
		t.Fatalf("expected Extra to include fileExtension")
	}
	var fileExt string
	if err := json.Unmarshal(cfg.Extra["fileExtension"], &fileExt); err != nil {
		t.Fatalf("failed to unmarshal fileExtension: %v", err)
	}
	if fileExt != "js" {
		t.Fatalf("expected fileExtension js, got %s", fileExt)
	}

	if _, err := os.Stat(filepath.Join(root, "Code.js")); err != nil {
		t.Fatalf("expected Code.js to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "appsscript.json")); err != nil {
		t.Fatalf("expected appsscript.json to exist: %v", err)
	}
}

func TestCloneCommandWithRootDirAndFileExtension(t *testing.T) {
	root := useTempDir(t)
	fake := &fakeScriptClient{
		getProjectResp: &script.Project{ParentId: "parent-id"},
		getContentResp: sampleContent(),
	}

	origWithCache := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCache })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}
	origConvert := convertPulledContentFn
	t.Cleanup(func() { convertPulledContentFn = origConvert })
	convertCalled := false
	convertPulledContentFn = func(content *script.Content, projectRoot string) (*script.Content, error) {
		convertCalled = true
		out := &script.Content{
			ScriptId: content.ScriptId,
			Files:    make([]*script.File, 0, len(content.Files)),
		}
		for _, f := range content.Files {
			if f == nil {
				continue
			}
			cloned := &script.File{Name: f.Name, Type: f.Type, Source: f.Source}
			if cloned.Type == "SERVER_JS" {
				cloned.Source = "converted ts source"
			}
			out.Files = append(out.Files, cloned)
		}
		return out, nil
	}

	cmd := CloneCmd{
		ScriptID:      "script-id",
		RootDir:       "src",
		FileExtension: "ts",
	}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CloneCmd.Run failed: %v", err)
	}
	if !convertCalled {
		t.Fatalf("expected pull-equivalent conversion for TypeScript extension")
	}

	cfg, err := config.LoadClaspConfig(root)
	if err != nil {
		t.Fatalf("LoadClaspConfig failed: %v", err)
	}
	if cfg.RootDir != "src" {
		t.Fatalf("expected RootDir src, got %s", cfg.RootDir)
	}
	var fileExt string
	if err := json.Unmarshal(cfg.Extra["fileExtension"], &fileExt); err != nil {
		t.Fatalf("failed to unmarshal fileExtension: %v", err)
	}
	if fileExt != "ts" {
		t.Fatalf("expected fileExtension ts, got %s", fileExt)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "Code.ts")); err != nil {
		t.Fatalf("expected src/Code.ts to exist: %v", err)
	}
	codeBytes, err := os.ReadFile(filepath.Join(root, "src", "Code.ts"))
	if err != nil {
		t.Fatalf("failed to read src/Code.ts: %v", err)
	}
	if !strings.Contains(string(codeBytes), "converted ts source") {
		t.Fatalf("expected converted TypeScript source to be written, got: %s", string(codeBytes))
	}
	if _, err := os.Stat(filepath.Join(root, "src", "appsscript.json")); err != nil {
		t.Fatalf("expected src/appsscript.json to exist: %v", err)
	}
}

func TestCloneCommandRejectsExistingConfig(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "existing"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	origWithCache := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCache })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cacheFile, authPath string) (scriptClient, error) {
		t.Fatal("expected clone to fail before client creation")
		return nil, nil
	}

	cmd := CloneCmd{ScriptID: "script-id"}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatal("expected error for existing .clasp.json")
	}
	if !strings.Contains(err.Error(), ".clasp.json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloneCommandRejectsTSXFileExtension(t *testing.T) {
	useTempDir(t)
	origWithCache := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCache })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected tsx validation to fail before client creation")
		return nil, nil
	}

	cmd := CloneCmd{ScriptID: "script-id", FileExtension: "tsx"}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected error for tsx fileExtension")
	}
	if !strings.Contains(err.Error(), `fileExtension "tsx" is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloneCommandPassesAuthPath(t *testing.T) {
	useTempDir(t)
	fake := &fakeScriptClient{
		getProjectResp: &script.Project{ParentId: "parent-id"},
		getContentResp: sampleContent(),
	}

	var gotAuthPath string
	origWithCache := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCache })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		gotAuthPath = authPath
		return fake, nil
	}

	cmd := CloneCmd{ScriptID: "script-id", Auth: " ./auth/.clasprc.json "}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CloneCmd.Run failed: %v", err)
	}
	if gotAuthPath != filepath.Clean("./auth/.clasprc.json") {
		t.Fatalf("expected cleaned auth path, got %q", gotAuthPath)
	}
}

func TestCloneCLIOptionUsesRootDir(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"clone", "script-id", "--rootDir", "src"})
	if err != nil {
		t.Fatalf("expected --rootDir to parse, got %v", err)
	}
	if cli.Clone.RootDir != "src" {
		t.Fatalf("expected rootDir src, got %q", cli.Clone.RootDir)
	}
}

func TestCloneCLIOptionRejectsLegacyRootDir(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	if _, err := parser.Parse([]string{"clone", "script-id", "--root-dir", "src"}); err == nil {
		t.Fatalf("expected --root-dir to be rejected")
	}
}
