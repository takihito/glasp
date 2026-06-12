package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/history"
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

func TestRecordRunHistoryStoresArchiveMetaAndCommandAlias(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	meta := runArchiveMeta{
		Enabled:   true,
		Direction: "pull",
		Path:      filepath.Join(root, ".glasp", "archive", "script-id", "pull", "20260308_120000"),
	}

	recordRunHistory([]string{"open", "--foo"}, "open", 15*time.Millisecond, nil, meta)

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

// captureStdout redirects the package-level stdout writer into a buffer for
// the duration of run.
func captureStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	orig := stdout
	stdout = &buf
	defer func() { stdout = orig }()
	runErr := run()
	return buf.String(), runErr
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
