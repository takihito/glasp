package main

import (
	"context"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"google.golang.org/api/script/v1"
)

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
