package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"google.golang.org/api/script/v1"
)

func TestCreateCommandFlow(t *testing.T) {
	root := useTempDir(t)
	fake := &fakeScriptClient{
		createProjectResp: &script.Project{ScriptId: "script-id", ParentId: "parent-id"},
		getContentResp:    sampleContent(),
	}

	var cacheFile string
	origWithCacheAuth := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCacheAuth })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		cacheFile = cachePath
		return fake, nil
	}

	cmd := CreateCmd{
		Title:         "My Project",
		RootDir:       "src",
		FileExtension: "gs",
	}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CreateCmd.Run failed: %v", err)
	}
	expectedCache := filepath.Join(root, ".glasp", "access.json")
	if cacheFile != expectedCache {
		t.Fatalf("expected cache path %s, got %s", expectedCache, cacheFile)
	}
	cfg, err := config.LoadClaspConfig(root)
	if err != nil {
		t.Fatalf("LoadClaspConfig failed: %v", err)
	}
	if cfg.ScriptID != "script-id" {
		t.Fatalf("expected ScriptID script-id, got %s", cfg.ScriptID)
	}
	if cfg.RootDir != "src" {
		t.Fatalf("expected RootDir src, got %s", cfg.RootDir)
	}

	if _, err := os.Stat(filepath.Join(root, "src", "Code.gs")); err != nil {
		t.Fatalf("expected Code.gs to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "appsscript.json")); err != nil {
		t.Fatalf("expected appsscript.json to exist: %v", err)
	}
}

func TestCreateCommandRejectsExistingConfig(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "existing"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	origWithCacheAuth := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCacheAuth })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cacheFile, authPath string) (scriptClient, error) {
		t.Fatal("expected create to fail before client creation")
		return nil, nil
	}

	cmd := CreateCmd{Title: "My Project"}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatal("expected error for existing .clasp.json")
	}
	if !strings.Contains(err.Error(), ".clasp.json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCommandRejectsNonStandaloneTypeWithoutParentID(t *testing.T) {
	useTempDir(t)
	origWithCacheAuth := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCacheAuth })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cacheFile, authPath string) (scriptClient, error) {
		t.Fatal("expected validation to fail before client creation")
		return nil, nil
	}

	cmd := CreateCmd{Title: "My Project", Type: "docs"}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected non-standalone type to be rejected without --parentId")
	}
	if !strings.Contains(err.Error(), "currently only \"standalone\" is supported without --parentId") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCommandPassesAuthPath(t *testing.T) {
	useTempDir(t)
	fake := &fakeScriptClient{
		createProjectResp: &script.Project{ScriptId: "script-id", ParentId: "parent-id"},
		getContentResp:    sampleContent(),
	}
	var gotAuthPath string
	origWithCacheAuth := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCacheAuth })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		gotAuthPath = authPath
		return fake, nil
	}

	cmd := CreateCmd{Title: "My Project", Auth: " ./auth/.clasprc.json "}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CreateCmd.Run failed: %v", err)
	}
	if gotAuthPath != filepath.Clean("./auth/.clasprc.json") {
		t.Fatalf("expected cleaned auth path, got %q", gotAuthPath)
	}
}

func TestCreateCommandRejectsEmptyAuthPath(t *testing.T) {
	useTempDir(t)
	origWithCacheAuth := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origWithCacheAuth })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	cmd := CreateCmd{Title: "My Project", Auth: "   "}
	if err := cmd.Run(nil); err == nil {
		t.Fatalf("expected error for empty --auth path")
	}
}

func TestCreateCLIOptionUsesCreateScriptName(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"create-script", "--title", "x"})
	if err != nil {
		t.Fatalf("expected create-script to parse, got %v", err)
	}
	if cli.CreateScript.Title != "x" {
		t.Fatalf("expected title x, got %q", cli.CreateScript.Title)
	}
}

func TestCreateCLIOptionDefaultFileExtensionIsJS(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"create-script", "--title", "x"})
	if err != nil {
		t.Fatalf("expected create-script to parse, got %v", err)
	}
	if cli.CreateScript.FileExtension != "js" {
		t.Fatalf("expected default fileExtension js, got %q", cli.CreateScript.FileExtension)
	}
}
