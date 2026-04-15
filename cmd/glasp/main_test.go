package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/history"
	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"
	"google.golang.org/api/script/v1"
)

type fakeScriptClient struct {
	createProjectResp *script.Project
	createProjectErr  error
	getProjectResp    *script.Project
	getProjectErr     error
	getContentResp    *script.Content
	getContentErr     error
	updateContentResp *script.Content
	updateContentErr  error
	createVersionResp *script.Version
	createVersionErr  error
	createDeployResp  *script.Deployment
	createDeployErr   error
	updateDeployResp  *script.Deployment
	updateDeployErr   error
	listDeployResp    []*script.Deployment
	listDeployErr     error
	runFunctionResp   *script.Operation
	runFunctionErr    error

	createProjectTitle  string
	createProjectParent string
	getProjectScriptID  string
	getContentScriptID  string
	updateScriptID      string
	updateContent       *script.Content
	createVersionID     string
	createVersionDesc   string
	createDeployID      string
	createDeployConfig  *script.DeploymentConfig
	updateDeployID      string
	updateDeploymentID  string
	updateDeployConfig  *script.DeploymentConfig
	listDeployScriptID  string
	runFunctionScriptID string
	runFunctionName     string
	runFunctionParams   []any
	runFunctionDevMode  bool

	createProjectCalled bool
	getProjectCalled    bool
	getContentCalled    bool
	updateContentCalled bool
	createVersionCalled bool
	createDeployCalled  bool
	updateDeployCalled  bool
	listDeployCalled    bool
	runFunctionCalled   bool
}

func (f *fakeScriptClient) CreateProject(ctx context.Context, title, parentID string) (*script.Project, error) {
	f.createProjectCalled = true
	f.createProjectTitle = title
	f.createProjectParent = parentID
	if f.createProjectErr != nil {
		return nil, f.createProjectErr
	}
	return f.createProjectResp, nil
}

func (f *fakeScriptClient) GetProject(ctx context.Context, scriptID string) (*script.Project, error) {
	f.getProjectCalled = true
	f.getProjectScriptID = scriptID
	if f.getProjectErr != nil {
		return nil, f.getProjectErr
	}
	return f.getProjectResp, nil
}

func (f *fakeScriptClient) GetContent(ctx context.Context, scriptID string, versionNumber int64) (*script.Content, error) {
	f.getContentCalled = true
	f.getContentScriptID = scriptID
	if f.getContentErr != nil {
		return nil, f.getContentErr
	}
	return f.getContentResp, nil
}

func (f *fakeScriptClient) UpdateContent(ctx context.Context, scriptID string, content *script.Content) (*script.Content, error) {
	f.updateContentCalled = true
	f.updateScriptID = scriptID
	f.updateContent = content
	if f.updateContentErr != nil {
		return nil, f.updateContentErr
	}
	return f.updateContentResp, nil
}

func (f *fakeScriptClient) CreateVersion(ctx context.Context, scriptID, description string) (*script.Version, error) {
	f.createVersionCalled = true
	f.createVersionID = scriptID
	f.createVersionDesc = description
	if f.createVersionErr != nil {
		return nil, f.createVersionErr
	}
	return f.createVersionResp, nil
}

func (f *fakeScriptClient) CreateDeployment(ctx context.Context, scriptID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error) {
	f.createDeployCalled = true
	f.createDeployID = scriptID
	f.createDeployConfig = deploymentConfig
	if f.createDeployErr != nil {
		return nil, f.createDeployErr
	}
	return f.createDeployResp, nil
}

func (f *fakeScriptClient) UpdateDeployment(ctx context.Context, scriptID, deploymentID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error) {
	f.updateDeployCalled = true
	f.updateDeployID = scriptID
	f.updateDeploymentID = deploymentID
	f.updateDeployConfig = deploymentConfig
	if f.updateDeployErr != nil {
		return nil, f.updateDeployErr
	}
	return f.updateDeployResp, nil
}

func (f *fakeScriptClient) ListDeployments(ctx context.Context, scriptID string) ([]*script.Deployment, error) {
	f.listDeployCalled = true
	f.listDeployScriptID = scriptID
	if f.listDeployErr != nil {
		return nil, f.listDeployErr
	}
	return f.listDeployResp, nil
}

func (f *fakeScriptClient) RunFunction(ctx context.Context, scriptID, functionName string, params []any, devMode bool) (*script.Operation, error) {
	f.runFunctionCalled = true
	f.runFunctionScriptID = scriptID
	f.runFunctionName = functionName
	f.runFunctionParams = params
	f.runFunctionDevMode = devMode
	if f.runFunctionErr != nil {
		return nil, f.runFunctionErr
	}
	return f.runFunctionResp, nil
}

func useTempDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to resolve current dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	return cwd
}

func sampleContent() *script.Content {
	return &script.Content{
		Files: []*script.File{
			{
				Name:   "Code",
				Type:   "SERVER_JS",
				Source: "function a() {}",
			},
			{
				Name:   "appsscript",
				Type:   "JSON",
				Source: "{}",
			},
		},
	}
}

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

func TestDeployAliasParsesAsUpdateDeployment(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"deploy", "dep-id", "--versionNumber", "8"})
	if err != nil {
		t.Fatalf("expected deploy alias to parse, got %v", err)
	}
	if cli.UpdateDeployment.DeploymentID != "dep-id" {
		t.Fatalf("expected deployment id dep-id, got %q", cli.UpdateDeployment.DeploymentID)
	}
	if cli.UpdateDeployment.Version != 8 {
		t.Fatalf("expected version 8, got %d", cli.UpdateDeployment.Version)
	}
}

func TestCreateDeploymentCLIParsesOptions(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"create-deployment", "--versionNumber", "4", "--description", "release", "--deploymentId", "dep-id"})
	if err != nil {
		t.Fatalf("expected create-deployment to parse, got %v", err)
	}
	if cli.CreateDeployment.Version != 4 {
		t.Fatalf("expected version 4, got %d", cli.CreateDeployment.Version)
	}
	if cli.CreateDeployment.Description != "release" {
		t.Fatalf("expected description release, got %q", cli.CreateDeployment.Description)
	}
	if cli.CreateDeployment.DeploymentID != "dep-id" {
		t.Fatalf("expected deploymentId dep-id, got %q", cli.CreateDeployment.DeploymentID)
	}
}

func TestListDeploymentsCLIParsesOptionalScriptID(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"list-deployments", "script-id"})
	if err != nil {
		t.Fatalf("expected list-deployments to parse, got %v", err)
	}
	if cli.ListDeployments.ScriptID != "script-id" {
		t.Fatalf("expected script-id, got %q", cli.ListDeployments.ScriptID)
	}
}

func TestRunFunctionCLIParsesOptions(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"run-function", "hello", "--nondev", "--params", `["a",1]`})
	if err != nil {
		t.Fatalf("expected run-function to parse, got %v", err)
	}
	if cli.RunFunction.FunctionName != "hello" {
		t.Fatalf("expected function hello, got %q", cli.RunFunction.FunctionName)
	}
	if !cli.RunFunction.NonDev {
		t.Fatalf("expected nondev true")
	}
	if cli.RunFunction.Params != `["a",1]` {
		t.Fatalf("expected params preserved, got %q", cli.RunFunction.Params)
	}
}

func TestHistoryCLIParsesOptions(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"history", "--limit", "5", "--status", "error", "--command", "pull", "--order", "asc"})
	if err != nil {
		t.Fatalf("expected history to parse, got %v", err)
	}
	if cli.History.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", cli.History.Limit)
	}
	if cli.History.Status != "error" {
		t.Fatalf("expected status error, got %q", cli.History.Status)
	}
	if cli.History.Command != "pull" {
		t.Fatalf("expected command pull, got %q", cli.History.Command)
	}
	if cli.History.Order != "asc" {
		t.Fatalf("expected order asc, got %q", cli.History.Order)
	}
}

func TestHistoryCommandNoFileReturnsNil(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	out, err := captureStdout(t, func() error { return (&HistoryCmd{}).Run(nil) })
	if err != nil {
		t.Fatalf("HistoryCmd.Run failed: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected [] output, got %q", out)
	}
}

func TestHistoryCommandFiltersByCommand(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "pull", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "push", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	entries, err := history.Read(root, history.ReadOptions{Command: "pull", Order: "asc"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Command != "pull" {
		t.Fatalf("unexpected filtered entries: %#v", entries)
	}
}

func TestHistoryCommandOutputsJSON(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "pull", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "push", Status: "error", Error: "boom"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	out, err := captureStdout(t, func() error {
		return (&HistoryCmd{Order: "asc"}).Run(nil)
	})
	if err != nil {
		t.Fatalf("HistoryCmd.Run failed: %v", err)
	}
	var entries []history.Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entries); err != nil {
		t.Fatalf("history output is not valid JSON: %v (out=%q)", err, out)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(entries))
	}
	if entries[0].Command != "pull" || entries[1].Command != "push" {
		t.Fatalf("unexpected history order/commands: %#v", entries)
	}
}

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

func TestOpenScriptCommandFlow(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	var openedURL string
	origOpen := openURLFn
	t.Cleanup(func() { openURLFn = origOpen })
	openURLFn = func(url string) error {
		openedURL = url
		return nil
	}

	if err := (&OpenScriptCmd{}).Run(nil); err != nil {
		t.Fatalf("OpenScriptCmd.Run failed: %v", err)
	}
	if openedURL != "https://script.google.com/d/script-id/edit" {
		t.Fatalf("unexpected opened URL: %s", openedURL)
	}
}

func TestOpenURLReturnsErrorWhenCommandFails(t *testing.T) {
	origGOOS := runtimeGOOS
	origExec := execCommandFn
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		execCommandFn = origExec
	})
	runtimeGOOS = "linux"
	execCommandFn = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 1")
	}

	err := openURL("https://example.com")
	if err == nil {
		t.Fatalf("expected error when open command exits non-zero")
	}
}

func TestOpenURLUsesWindowsRundll32(t *testing.T) {
	origGOOS := runtimeGOOS
	origExec := execCommandFn
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		execCommandFn = origExec
	})
	runtimeGOOS = "windows"
	var gotName string
	var gotArgs []string
	execCommandFn = func(name string, arg ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), arg...)
		return exec.Command("true")
	}

	if err := openURL("https://example.com"); err != nil {
		t.Fatalf("openURL failed: %v", err)
	}
	if gotName != "rundll32" {
		t.Fatalf("expected rundll32, got %s", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "url.dll,FileProtocolHandler" || gotArgs[1] != "https://example.com" {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
}

func TestUpdateDeploymentCommandFlowWithVersion(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 7,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "dep-id", Version: 7, Description: "hotfix"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("UpdateDeploymentCmd.Run failed: %v", err)
	}
	if fake.createVersionCalled {
		t.Fatalf("did not expect CreateVersion to be called when --versionNumber is provided")
	}
	if !fake.updateDeployCalled {
		t.Fatalf("expected UpdateDeployment to be called")
	}
	if fake.updateDeployConfig == nil || fake.updateDeployConfig.VersionNumber != 7 {
		t.Fatalf("unexpected deployment config: %#v", fake.updateDeployConfig)
	}
}

func TestUpdateDeploymentCommandFlowCreatesVersionWhenMissing(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		createVersionResp: &script.Version{VersionNumber: 9},
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 9,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "dep-id", Description: "release"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("UpdateDeploymentCmd.Run failed: %v", err)
	}
	if !fake.createVersionCalled {
		t.Fatalf("expected CreateVersion to be called when --versionNumber is not provided")
	}
	if fake.createVersionID != "script-id" {
		t.Fatalf("expected script-id for version creation, got %s", fake.createVersionID)
	}
	if fake.updateDeployConfig == nil || fake.updateDeployConfig.VersionNumber != 9 {
		t.Fatalf("unexpected deployment config: %#v", fake.updateDeployConfig)
	}
}

func TestUpdateDeploymentCommandRejectsMissingDeploymentID(t *testing.T) {
	useTempDir(t)
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	cmd := UpdateDeploymentCmd{}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected error for missing deployment ID")
	}
	if !strings.Contains(err.Error(), "deployment ID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDeploymentCommandRejectsBlankDeploymentID(t *testing.T) {
	useTempDir(t)
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "   "}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected error for blank deployment ID")
	}
	if !strings.Contains(err.Error(), "deployment ID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDeploymentCommandFailsWhenEntryPointsMarshalFails(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 7,
			},
		},
	}
	origClient := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origClient })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}
	origMarshal := marshalJSONFn
	t.Cleanup(func() { marshalJSONFn = origMarshal })
	marshalJSONFn = func(v any) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "dep-id", Version: 7}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected error when entryPoints marshal fails")
	}
	if !strings.Contains(err.Error(), "failed to marshal deployment entry points") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateDeploymentCommandFlowCreatesNewDeployment(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		createVersionResp: &script.Version{VersionNumber: 11},
		createDeployResp: &script.Deployment{
			DeploymentId: "dep-new",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 11,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := CreateDeploymentCmd{Description: "release"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CreateDeploymentCmd.Run failed: %v", err)
	}
	if !fake.createVersionCalled {
		t.Fatalf("expected CreateVersion to be called when version is omitted")
	}
	if !fake.createDeployCalled {
		t.Fatalf("expected CreateDeployment to be called")
	}
	if fake.createDeployConfig == nil || fake.createDeployConfig.VersionNumber != 11 {
		t.Fatalf("unexpected deployment config: %#v", fake.createDeployConfig)
	}
}

func TestCreateDeploymentCommandWithDeploymentIDUsesUpdate(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 7,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := CreateDeploymentCmd{DeploymentID: "dep-id", Version: 7, Description: "redeploy"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CreateDeploymentCmd.Run failed: %v", err)
	}
	if fake.createDeployCalled {
		t.Fatalf("did not expect CreateDeployment when deploymentId is provided")
	}
	if !fake.updateDeployCalled {
		t.Fatalf("expected UpdateDeployment to be called")
	}
}

func TestListDeploymentsCommandUsesConfigScriptID(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		listDeployResp: []*script.Deployment{
			{
				DeploymentId: "dep-1",
				DeploymentConfig: &script.DeploymentConfig{
					VersionNumber: 2,
				},
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	if err := (&ListDeploymentsCmd{}).Run(nil); err != nil {
		t.Fatalf("ListDeploymentsCmd.Run failed: %v", err)
	}
	if !fake.listDeployCalled {
		t.Fatalf("expected ListDeployments to be called")
	}
	if fake.listDeployScriptID != "script-id" {
		t.Fatalf("expected script-id, got %s", fake.listDeployScriptID)
	}
}

func TestRunFunctionCommandFlow(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		runFunctionResp: &script.Operation{
			Done:     true,
			Response: []byte(`{"result":"ok"}`),
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := RunFunctionCmd{FunctionName: "hello", Params: `["a",1]`}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("RunFunctionCmd.Run failed: %v", err)
	}
	if !fake.runFunctionCalled {
		t.Fatalf("expected RunFunction to be called")
	}
	if fake.runFunctionScriptID != "script-id" {
		t.Fatalf("expected script-id, got %s", fake.runFunctionScriptID)
	}
	if fake.runFunctionName != "hello" {
		t.Fatalf("expected function hello, got %s", fake.runFunctionName)
	}
	if !fake.runFunctionDevMode {
		t.Fatalf("expected devMode=true by default")
	}
	if len(fake.runFunctionParams) != 2 {
		t.Fatalf("unexpected params: %#v", fake.runFunctionParams)
	}
}

func TestRunFunctionCommandRejectsUnfinishedOperation(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		runFunctionResp: &script.Operation{
			Done: false,
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := RunFunctionCmd{FunctionName: "hello"}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected unfinished operation error")
	}
	if !strings.Contains(err.Error(), "still in progress") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFunctionCommandRejectsNonJSONArrayParams(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	cmd := RunFunctionCmd{FunctionName: "hello", Params: `{"k":"v"}`}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected params validation error")
	}
	if !strings.Contains(err.Error(), "params must be a JSON array") {
		t.Fatalf("unexpected error: %v", err)
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

func TestPushCommandFlow(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Force: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if !fake.updateContentCalled {
		t.Fatal("expected UpdateContent to be called")
	}
	if fake.updateScriptID != "script-id" {
		t.Fatalf("expected script-id, got %s", fake.updateScriptID)
	}
	if file := findContentFile(fake.updateContent, "Code"); file == nil || file.Type != "SERVER_JS" {
		t.Fatalf("expected Code SERVER_JS, got %#v", file)
	}
	if file := findContentFile(fake.updateContent, "appsscript"); file == nil || file.Type != "JSON" {
		t.Fatalf("expected appsscript JSON, got %#v", file)
	}
}

func TestPushCommandPassesAuthPath(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{}
	var gotAuthPath string
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		gotAuthPath = authPath
		return fake, nil
	}

	if err := (&PushCmd{Force: true, Auth: " ./auth/.clasprc.json "}).Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
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

func TestPushCommandDryRunSkipsAPIAndArchive(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected dryrun push to skip client creation")
		return nil, nil
	}

	cmd := PushCmd{Force: true, Archive: true, DryRun: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".glasp", "archive")); !os.IsNotExist(err) {
		t.Fatalf("expected archive not to be created during dryrun")
	}
}

func TestPushCLIParsesHistoryID(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"push", "--history-id", "42"})
	if err != nil {
		t.Fatalf("expected push --history-id to parse, got %v", err)
	}
	if cli.Push.HistoryID != 42 {
		t.Fatalf("expected history-id 42, got %d", cli.Push.HistoryID)
	}
}

func TestPushCommandHistoryIDUsesArchivePayload(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120000")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120000","fileExtension":"js","convert":"none","status":"success"}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{HistoryID: 1}
	out, err := captureStdout(t, func() error { return cmd.Run(nil) })
	if err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if !fake.updateContentCalled {
		t.Fatalf("expected UpdateContent to be called")
	}
	if !strings.Contains(out, "Using history source id=1") {
		t.Fatalf("expected history source output, got %q", out)
	}
	if !strings.Contains(out, "manifest.timestamp=20260309_120000") {
		t.Fatalf("expected manifest timestamp output, got %q", out)
	}
	if file := findContentFile(fake.updateContent, "Code"); file == nil || file.Type != "SERVER_JS" {
		t.Fatalf("expected Code SERVER_JS in replay payload, got %#v", file)
	}
}

func TestPushCommandHistoryIDDryRunSkipsAPI(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120001")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120001","fileExtension":"js","convert":"none","status":"success"}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected dryrun history push to skip client creation")
		return nil, nil
	}

	out, err := captureStdout(t, func() error { return (&PushCmd{HistoryID: 1, DryRun: true}).Run(nil) })
	if err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if !strings.Contains(out, "Using history source id=1") {
		t.Fatalf("expected history source output, got %q", out)
	}
	if !strings.Contains(out, "manifest.timestamp=20260309_120001") {
		t.Fatalf("expected manifest timestamp output, got %q", out)
	}
}

func TestArchivePushRunWritesPayloadIndex(t *testing.T) {
	root := t.TempDir()
	workingFiles := []syncer.ProjectFile{
		{LocalPath: "src/Code.ts", RemotePath: "Code", Type: "SERVER_JS", Source: "const x = 1"},
	}
	payloadFiles := []syncer.ProjectFile{
		{LocalPath: "src/Code.js", RemotePath: "Code", Type: "SERVER_JS", Source: "function x() {}"},
		{LocalPath: "appsscript.json", RemotePath: "appsscript", Type: "JSON", Source: "{}"},
	}
	archiveRoot, err := archivePushRun(root, "script-id", workingFiles, payloadFiles, "ts", transform.ModeTSToGas)
	if err != nil {
		t.Fatalf("archivePushRun failed: %v", err)
	}
	manifestPath := filepath.Join(archiveRoot, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest failed: %v", err)
	}
	var manifest archiveManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest failed: %v", err)
	}
	if len(manifest.PayloadIndex) != 2 {
		t.Fatalf("expected 2 payloadIndex entries, got %d", len(manifest.PayloadIndex))
	}
	if manifest.PayloadIndex[0].Path != "appsscript.json" || manifest.PayloadIndex[0].RemotePath != "appsscript" {
		t.Fatalf("unexpected first payloadIndex entry: %#v", manifest.PayloadIndex[0])
	}
	if manifest.PayloadIndex[1].Path != "src/Code.js" || manifest.PayloadIndex[1].RemotePath != "Code" {
		t.Fatalf("unexpected second payloadIndex entry: %#v", manifest.PayloadIndex[1])
	}
}

func TestPushCommandHistoryIDLegacyArchiveStripsRootDirPrefix(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120002")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload", "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120002","fileExtension":"js","convert":"none","status":"success"}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "src", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	if err := (&PushCmd{HistoryID: 1}).Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if file := findContentFile(fake.updateContent, "Code"); file == nil {
		t.Fatalf("expected remote path Code to be used, got %#v", fake.updateContent)
	}
	if file := findContentFile(fake.updateContent, "src/Code"); file != nil {
		t.Fatalf("expected src/Code not to be used, got %#v", file)
	}
}

func TestPushCommandHistoryIDPrefersManifestPayloadIndexRemotePath(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120003")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload", "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120003","fileExtension":"js","convert":"none","status":"success","payloadIndex":[{"path":"src/Code.js","remotePath":"SpecialCode","type":"SERVER_JS"}]}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "src", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	if err := (&PushCmd{HistoryID: 1}).Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if file := findContentFile(fake.updateContent, "SpecialCode"); file == nil {
		t.Fatalf("expected payloadIndex remote path SpecialCode, got %#v", fake.updateContent)
	}
}

func TestPushCommandHistoryIDErrorsWhenMissing(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	err := (&PushCmd{HistoryID: 99}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for missing history id")
	}
	if !strings.Contains(err.Error(), "history id 99 not found") {
		t.Fatalf("unexpected error: %v", err)
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

func TestPushCommandDryRunRejectsEmptyAuthPath(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	err := (&PushCmd{Force: true, DryRun: true, Auth: "   "}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for empty --auth path in dryrun push")
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

func TestPushCommandRejectsTSXFileExtension(t *testing.T) {
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

	err := (&PushCmd{}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for tsx fileExtension")
	}
	if !strings.Contains(err.Error(), `fileExtension "tsx" is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConvertCommandRequiresMode(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	err := (&ConvertCmd{}).Run(nil)
	if err == nil {
		t.Fatalf("expected error when no mode is specified")
	}
	if !strings.Contains(err.Error(), "either --gas-to-ts or --ts-to-gas") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConvertCommandFlow(t *testing.T) {
	root := useTempDir(t)
	cfg := &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	origConvert := transformConvertFn
	t.Cleanup(func() { transformConvertFn = origConvert })
	var gotOutDir string
	var gotMode transform.Mode
	var gotFilter *transform.TargetFilter
	transformConvertFn = func(opts syncer.Options, outDir string, mode transform.Mode, filter *transform.TargetFilter) (transform.Result, error) {
		gotOutDir = outDir
		gotMode = mode
		gotFilter = filter
		return transform.Result{OutDir: outDir}, nil
	}

	cmd := ConvertCmd{GasToTS: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("ConvertCmd.Run failed: %v", err)
	}
	expectedOut := filepath.Join(root, ".glasp", "dist", "ts")
	if gotOutDir != expectedOut {
		t.Fatalf("expected out dir %s, got %s", expectedOut, gotOutDir)
	}
	if gotMode != transform.ModeGasToTS {
		t.Fatalf("expected mode gas-to-ts, got %s", gotMode)
	}
	if gotFilter != nil {
		t.Fatalf("expected no target filter, got %#v", gotFilter)
	}
}

func TestRecordRunHistoryStoresArchiveMetaAndCommandAlias(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	resetRunArchiveMeta()
	setRunArchiveMeta(true, "pull")
	setRunArchivePath(filepath.Join(root, ".glasp", "archive", "script-id", "pull", "20260308_120000"))

	recordRunHistory([]string{"open", "--foo"}, "open", 15*time.Millisecond, nil)

	entries, err := history.Read(root, history.ReadOptions{Order: "asc"})
	if err != nil {
		t.Fatalf("history.Read failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one history entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Command != "open" {
		t.Fatalf("expected alias command open, got %q", entry.Command)
	}
	if !entry.Archive.Enabled || entry.Archive.Direction != "pull" {
		t.Fatalf("unexpected archive metadata: %#v", entry.Archive)
	}
	if entry.Archive.Path == "" {
		t.Fatalf("expected archive path to be populated")
	}
}

func findContentFile(content *script.Content, name string) *script.File {
	if content == nil {
		return nil
	}
	for _, file := range content.Files {
		if file != nil && file.Name == name {
			return file
		}
	}
	return nil
}

func captureStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w
	runErr := run()
	_ = w.Close()
	os.Stdout = orig
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read stdout failed: %v", readErr)
	}
	_ = r.Close()
	return string(out), runErr
}

func TestValidateTitle(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if _, err := validateTitle(""); err == nil {
			t.Fatalf("expected error for empty title")
		}
	})
	t.Run("whitespace", func(t *testing.T) {
		if _, err := validateTitle("   "); err == nil {
			t.Fatalf("expected error for whitespace title")
		}
	})
	t.Run("too-long", func(t *testing.T) {
		title := strings.Repeat("a", maxTitleLength+1)
		if _, err := validateTitle(title); err == nil {
			t.Fatalf("expected error for long title")
		}
	})
	t.Run("control-chars", func(t *testing.T) {
		if _, err := validateTitle("hello\nworld"); err == nil {
			t.Fatalf("expected error for control character in title")
		}
	})
	t.Run("valid", func(t *testing.T) {
		got, err := validateTitle("  My Title  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "My Title" {
			t.Fatalf("expected trimmed title, got %q", got)
		}
	})
}

func TestValidateScriptID(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if _, err := validateScriptID(""); err == nil {
			t.Fatalf("expected error for empty script ID")
		}
	})
	t.Run("whitespace", func(t *testing.T) {
		if _, err := validateScriptID("  "); err == nil {
			t.Fatalf("expected error for whitespace script ID")
		}
	})
	t.Run("invalid-format", func(t *testing.T) {
		if _, err := validateScriptID("abc/def"); err == nil {
			t.Fatalf("expected error for invalid script ID")
		}
	})
	t.Run("valid", func(t *testing.T) {
		got, err := validateScriptID("  abc_DEF-123  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "abc_DEF-123" {
			t.Fatalf("expected trimmed script ID, got %q", got)
		}
	})
}

func TestCommandFromArgs(t *testing.T) {
	t.Run("nested config subcommand", func(t *testing.T) {
		got := commandFromArgs([]string{"config", "init"})
		if got != "config init" {
			t.Fatalf("expected config init, got %q", got)
		}
	})
	t.Run("alias is preserved as entered", func(t *testing.T) {
		got := commandFromArgs([]string{"open", "--scriptId", "abc"})
		if got != "open" {
			t.Fatalf("expected open, got %q", got)
		}
	})
	t.Run("single command remains unchanged", func(t *testing.T) {
		got := commandFromArgs([]string{"pull", "--dryrun"})
		if got != "pull" {
			t.Fatalf("expected pull, got %q", got)
		}
	})
}

func TestSanitizeHistoryArgs(t *testing.T) {
	args := []string{
		"run-function",
		"--auth", "/tmp/.clasprc.json",
		"--params", `["token-value"]`,
		"--token=abc123",
		"--api-key=xyz",
		"--my-auth", "abc",
		"--auth-token=def",
		"--service-key", "ghi",
		"--password", "p@ss",
		"--secret", "s3",
		"--normal", "ok",
	}
	got := sanitizeHistoryArgs(args)
	want := []string{
		"run-function",
		"--auth", "REDACTED",
		"--params", "REDACTED",
		"--token=REDACTED",
		"--api-key=REDACTED",
		"--my-auth", "REDACTED",
		"--auth-token=REDACTED",
		"--service-key", "REDACTED",
		"--password", "REDACTED",
		"--secret", "REDACTED",
		"--normal", "ok",
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] mismatch: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestSanitizeHistoryArgsShortFlags(t *testing.T) {
	args := []string{
		"run-function",
		"myFunc",
		"-p", `["secret-value"]`,
		"--nondev",
	}
	got := sanitizeHistoryArgs(args)
	want := []string{
		"run-function",
		"myFunc",
		"-p", "REDACTED",
		"--nondev",
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] mismatch: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestCLIDirFlagParsed(t *testing.T) {
	var cli CLI
	p, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	if _, err = p.Parse([]string{"--dir", "/tmp", "version"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cli.Dir != "/tmp" {
		t.Fatalf("expected Dir=/tmp, got %q", cli.Dir)
	}
}

func TestCLIDirEnvVar(t *testing.T) {
	t.Setenv("GLASP_DIR", "/tmp")
	var cli CLI
	p, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	if _, err = p.Parse([]string{"version"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cli.Dir != "/tmp" {
		t.Fatalf("expected Dir=/tmp from GLASP_DIR, got %q", cli.Dir)
	}
}

func TestCLIDirShortFlag(t *testing.T) {
	var cli CLI
	p, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("kong.New failed: %v", err)
	}
	if _, err = p.Parse([]string{"-C", "/tmp", "version"}); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cli.Dir != "/tmp" {
		t.Fatalf("expected Dir=/tmp via -C, got %q", cli.Dir)
	}
}

func TestSanitizeHistoryArgsDirNotRedacted(t *testing.T) {
	args := []string{"push", "--dir", "/workspace/gas", "--auth", "/tmp/.clasprc.json"}
	got := sanitizeHistoryArgs(args)
	if got[2] == "REDACTED" {
		t.Fatalf("--dir value should not be redacted, but was")
	}
	if got[2] != "/workspace/gas" {
		t.Fatalf("expected --dir value preserved as /workspace/gas, got %q", got[2])
	}
	// --auth should still be redacted
	if got[4] != "REDACTED" {
		t.Fatalf("expected --auth value to be redacted, got %q", got[4])
	}
}

func TestFindExistingProjectRootFindsParent(t *testing.T) {
	// Setup: create a temp project root with .clasp.json, then cd into a subdir
	projectRoot := t.TempDir()
	claspJSON := filepath.Join(projectRoot, ".clasp.json")
	if err := os.WriteFile(claspJSON, []byte(`{"scriptId":"abc123"}`), 0644); err != nil {
		t.Fatalf("failed to write .clasp.json: %v", err)
	}
	subdir := filepath.Join(projectRoot, "src", "nested")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var buf strings.Builder
	origStdout := stdout
	stdout = &buf
	t.Cleanup(func() { stdout = origStdout })

	got, err := findExistingProjectRoot()
	if err != nil {
		t.Fatalf("findExistingProjectRoot failed: %v", err)
	}
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(projectRoot)
	if gotResolved != wantResolved {
		t.Fatalf("expected project root %q, got %q", wantResolved, gotResolved)
	}
	// CWD != project root → should print "Project root: ..."
	if !strings.Contains(buf.String(), "Project root:") {
		t.Fatalf("expected 'Project root:' in output, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), got) {
		t.Fatalf("expected output to contain resolved path %q, got %q", got, buf.String())
	}
}

func TestFindExistingProjectRootNoOutputWhenAtRoot(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, ".clasp.json"), []byte(`{"scriptId":"abc"}`), 0644); err != nil {
		t.Fatalf("failed to write .clasp.json: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	var buf strings.Builder
	origStdout := stdout
	stdout = &buf
	t.Cleanup(func() { stdout = origStdout })

	if _, err := findExistingProjectRoot(); err != nil {
		t.Fatalf("findExistingProjectRoot failed: %v", err)
	}
	// CWD == project root → no output expected
	if buf.Len() != 0 {
		t.Fatalf("expected no output when already at project root, got %q", buf.String())
	}
}

func TestFindExistingProjectRootNotFound(t *testing.T) {
	// Setup: a temp dir with no .clasp.json anywhere above it
	emptyDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	if err := os.Chdir(emptyDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	_, err = findExistingProjectRoot()
	if err == nil {
		t.Fatalf("expected error when .clasp.json is not found")
	}
	if !strings.Contains(err.Error(), ".clasp.json not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
