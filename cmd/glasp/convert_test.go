package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"
)

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
