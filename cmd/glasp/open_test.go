package main

import (
	"os/exec"
	"testing"

	"github.com/takihito/glasp/internal/config"
)

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
