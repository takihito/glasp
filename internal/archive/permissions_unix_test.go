//go:build !windows

package archive

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"

	"google.golang.org/api/script/v1"
)

// withPermissiveUmask clears the process umask for the duration of the test
// so the assertions below verify the modes glasp requests rather than ones
// a restrictive developer/CI umask happens to produce.
func withPermissiveUmask(t *testing.T) {
	t.Helper()
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })
}

// assertOwnerOnly walks the per-script archive tree and fails on any entry
// readable/writable by group or other (threat model M13).
func assertOwnerOnly(t *testing.T, projectRoot string) {
	t.Helper()
	archiveDir := filepath.Join(projectRoot, ".glasp", "archive")
	seen := 0
	err := filepath.WalkDir(archiveDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		seen++
		want := os.FileMode(0o600)
		if d.IsDir() {
			want = 0o700
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: mode %#o, want %#o", p, got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk archive: %v", err)
	}
	if seen == 0 {
		t.Fatalf("no archive entries found under %s", archiveDir)
	}
}

func TestPushRunCreatesOwnerOnlyArchive(t *testing.T) {
	withPermissiveUmask(t)
	root := t.TempDir()
	files := []syncer.ProjectFile{
		{LocalPath: "src/lib/Code.js", RemotePath: "lib/Code", Type: "SERVER_JS", Source: "function x() {}"},
		{LocalPath: "appsscript.json", RemotePath: "appsscript", Type: "JSON", Source: "{}"},
	}
	if _, err := PushRun(root, "script-id", files, files, "js", transform.Mode("")); err != nil {
		t.Fatalf("PushRun failed: %v", err)
	}
	assertOwnerOnly(t, root)
}

func TestPullRunCreatesOwnerOnlyArchive(t *testing.T) {
	withPermissiveUmask(t)
	root := t.TempDir()
	cfg := &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}
	content := &script.Content{Files: []*script.File{
		{Name: "lib/Code", Type: "SERVER_JS", Source: "function x() {}"},
		{Name: "appsscript", Type: "JSON", Source: "{}"},
	}}
	if _, err := PullRun(root, "script-id", cfg, content, content, "js", transform.Mode("")); err != nil {
		t.Fatalf("PullRun failed: %v", err)
	}
	assertOwnerOnly(t, root)
}
