package browser

import (
	"os/exec"
	"testing"
)

func patchSeams(t *testing.T, os string, fn func(name string, arg ...string) *exec.Cmd) {
	t.Helper()
	origGOOS := goos
	origExec := execCommand
	t.Cleanup(func() {
		goos = origGOOS
		execCommand = origExec
	})
	goos = os
	execCommand = fn
}

func TestOpenReturnsErrorWhenCommandFails(t *testing.T) {
	patchSeams(t, "linux", func(name string, arg ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 1")
	})

	if err := Open("https://example.com"); err == nil {
		t.Fatalf("expected error when open command exits non-zero")
	}
}

func TestOpenUsesWindowsRundll32(t *testing.T) {
	var gotName string
	var gotArgs []string
	patchSeams(t, "windows", func(name string, arg ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), arg...)
		return exec.Command("true")
	})

	if err := Open("https://example.com"); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if gotName != "rundll32" {
		t.Fatalf("expected rundll32, got %s", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "url.dll,FileProtocolHandler" || gotArgs[1] != "https://example.com" {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
}

func TestOpenRejectsEmptyURL(t *testing.T) {
	if err := Open("   "); err == nil {
		t.Fatalf("expected error for empty url")
	}
}

func TestOpenRejectsUnsupportedPlatform(t *testing.T) {
	patchSeams(t, "plan9", func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	})
	if err := Open("https://example.com"); err == nil {
		t.Fatalf("expected error for unsupported platform")
	}
}

func TestStartUsesDarwinOpen(t *testing.T) {
	var gotName string
	patchSeams(t, "darwin", func(name string, arg ...string) *exec.Cmd {
		gotName = name
		return exec.Command("true")
	})
	if err := Start("https://example.com"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if gotName != "open" {
		t.Fatalf("expected open, got %s", gotName)
	}
}
