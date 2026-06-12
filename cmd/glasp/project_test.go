package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	origStderr := stderr
	stderr = &buf
	t.Cleanup(func() { stderr = origStderr })

	got, err := findExistingProjectRoot()
	if err != nil {
		t.Fatalf("findExistingProjectRoot failed: %v", err)
	}
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(projectRoot)
	if gotResolved != wantResolved {
		t.Fatalf("expected project root %q, got %q", wantResolved, gotResolved)
	}
	// CWD != project root → should print "Project root: ..." to stderr
	if !strings.Contains(buf.String(), "Project root:") {
		t.Fatalf("expected 'Project root:' in stderr, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), got) {
		t.Fatalf("expected stderr to contain resolved path %q, got %q", got, buf.String())
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
	origStderr := stderr
	stderr = &buf
	t.Cleanup(func() { stderr = origStderr })

	if _, err := findExistingProjectRoot(); err != nil {
		t.Fatalf("findExistingProjectRoot failed: %v", err)
	}
	// CWD == project root → no output expected
	if buf.Len() != 0 {
		t.Fatalf("expected no stderr when already at project root, got %q", buf.String())
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
