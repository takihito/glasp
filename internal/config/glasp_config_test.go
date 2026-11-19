package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGlaspConfigNotFoundReturnsDefault(t *testing.T) {
	root := t.TempDir()
	cfg, err := LoadGlaspConfig(root)
	if err != nil {
		t.Fatalf("LoadGlaspConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Archive.Pull {
		t.Fatal("expected archive.pull to be false by default")
	}
	if cfg.Archive.Push {
		t.Fatal("expected archive.push to be false by default")
	}
}

func TestSaveLoadGlaspConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	original := &GlaspConfig{Archive: ArchiveConfig{Pull: true, Push: true}}
	if err := SaveGlaspConfig(root, original); err != nil {
		t.Fatalf("SaveGlaspConfig failed: %v", err)
	}
	loaded, err := LoadGlaspConfig(root)
	if err != nil {
		t.Fatalf("LoadGlaspConfig failed: %v", err)
	}
	if loaded.Archive.Pull != original.Archive.Pull {
		t.Fatalf("archive.pull mismatch: expected %v, got %v", original.Archive.Pull, loaded.Archive.Pull)
	}
	if loaded.Archive.Push != original.Archive.Push {
		t.Fatalf("archive.push mismatch: expected %v, got %v", original.Archive.Push, loaded.Archive.Push)
	}
}

func TestEnsureGlaspDirCreatesClaspignore(t *testing.T) {
	root := t.TempDir()

	// No .claspignore exists yet — EnsureGlaspDir should create it with .glasp/ entry.
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("expected .claspignore to be created: %v", err)
	}
	if !strings.Contains(string(data), ".glasp/") {
		t.Errorf("expected .claspignore to contain '.glasp/', got: %q", string(data))
	}

	// Calling again should not duplicate the entry.
	// Remove .glasp dir to trigger the "created" path again.
	os.RemoveAll(filepath.Join(root, ".glasp"))
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir (2nd call) failed: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("failed to read .claspignore: %v", err)
	}
	if strings.Count(string(data), ".glasp/") != 1 {
		t.Errorf("expected exactly one '.glasp/' entry, got: %q", string(data))
	}
}

func TestEnsureGlaspDirExistingClaspignore(t *testing.T) {
	root := t.TempDir()

	// Pre-existing .claspignore without trailing newline.
	if err := os.WriteFile(filepath.Join(root, ".claspignore"), []byte("node_modules/"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("failed to read .claspignore: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, ".glasp/") {
		t.Errorf("expected .glasp/ to be appended, got: %q", content)
	}
	if !strings.Contains(content, "node_modules/") {
		t.Errorf("expected original content to be preserved, got: %q", content)
	}
	// Should have a newline between original content and new entry.
	if strings.Contains(content, "node_modules/.glasp/") {
		t.Errorf("expected newline separator, got: %q", content)
	}
}

func TestEnsureGlaspDirAlreadyHasEntry(t *testing.T) {
	root := t.TempDir()

	// .claspignore already contains .glasp/
	if err := os.WriteFile(filepath.Join(root, ".claspignore"), []byte(".glasp/\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".claspignore"))
	if err != nil {
		t.Fatalf("failed to read .claspignore: %v", err)
	}
	if strings.Count(string(data), ".glasp/") != 1 {
		t.Errorf("expected no duplicate, got: %q", string(data))
	}
}

func TestLoadGlaspConfigInvalidJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, glaspConfigDirName), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, glaspConfigDirName, glaspConfigFileName), []byte("{"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := LoadGlaspConfig(root); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEnsureGlaspDirCreatesGitignore(t *testing.T) {
	root := t.TempDir()

	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".glasp", ".gitignore"))
	if err != nil {
		t.Fatalf("expected .glasp/.gitignore to be created: %v", err)
	}
	if string(data) != glaspGitignoreContent {
		t.Errorf("unexpected .glasp/.gitignore content: %q", string(data))
	}
	// The project-root .gitignore must not be created or touched.
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("expected no project-root .gitignore, stat err: %v", err)
	}
}

func TestEnsureGlaspDirAddsGitignoreToExistingDir(t *testing.T) {
	root := t.TempDir()

	// Simulate a .glasp/ directory created by an older glasp version.
	if err := os.MkdirAll(filepath.Join(root, ".glasp"), 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".glasp", "access.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".glasp", ".gitignore"))
	if err != nil {
		t.Fatalf("expected .glasp/.gitignore to be created for existing dir: %v", err)
	}
	if string(data) != glaspGitignoreContent {
		t.Errorf("unexpected .glasp/.gitignore content: %q", string(data))
	}
	// The "created" path did not run, so .claspignore must stay untouched.
	if _, err := os.Stat(filepath.Join(root, ".claspignore")); !os.IsNotExist(err) {
		t.Errorf("expected .claspignore not to be created for existing dir, stat err: %v", err)
	}
}

func TestEnsureGlaspDirKeepsCustomGitignore(t *testing.T) {
	root := t.TempDir()

	custom := "*\n!config.json\n!my-notes.md\n"
	if err := os.MkdirAll(filepath.Join(root, ".glasp"), 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	path := filepath.Join(root, ".glasp", ".gitignore")
	if err := os.WriteFile(path, []byte(custom), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != custom {
		t.Errorf("expected custom .glasp/.gitignore to be preserved, got: %q", string(data))
	}
}

func TestGlaspGitignoreExcludesSecretsFromGit(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not found in PATH")
	}
	root := t.TempDir()
	// Isolate from the developer's global/system git config (e.g. a global
	// excludesFile) so only .glasp/.gitignore decides the outcome.
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+root,
	)
	run := func(args ...string) error {
		cmd := exec.Command(gitPath, args...)
		cmd.Dir = root
		cmd.Env = env
		return cmd.Run()
	}
	if err := run("init", "-q"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	files := map[string]bool{ // path -> expected to be ignored
		".glasp/access.json":      true,
		".glasp/history.jsonl":    true,
		".glasp/history.lock":     true,
		".glasp/archive/x/a.json": true,
		".glasp/future-file":      true,
		".glasp/config.json":      false,
		".glasp/.gitignore":       false,
	}
	for rel := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if rel == ".glasp/.gitignore" {
			continue // already created by EnsureGlaspDir
		}
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
	for rel, wantIgnored := range files {
		// git check-ignore exits 0 when the path is ignored, 1 when it is not.
		err := run("check-ignore", "-q", rel)
		ignored := err == nil
		if exitErr, ok := err.(*exec.ExitError); err != nil && (!ok || exitErr.ExitCode() != 1) {
			t.Fatalf("git check-ignore %s failed: %v", rel, err)
		}
		if ignored != wantIgnored {
			t.Errorf("%s: ignored=%v, want %v", rel, ignored, wantIgnored)
		}
	}
}
