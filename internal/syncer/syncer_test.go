package syncer

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/takihito/glasp/internal/config"
	"google.golang.org/api/script/v1"
)

func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(orig)
	fn()
	return buf.String()
}

func TestOptionsFromConfigFileExtension(t *testing.T) {
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		RootDir:  "src",
		Extra: map[string]json.RawMessage{
			"fileExtension":        json.RawMessage(`"ts"`),
			"ignoreSubdirectories": json.RawMessage(`true`),
		},
	}
	opts, err := OptionsFromConfig("/tmp/project", cfg, nil)
	if err != nil {
		t.Fatalf("OptionsFromConfig failed: %v", err)
	}
	if opts.FileExtensions[FileTypeServerJS][0] != ".ts" {
		t.Fatalf("expected .ts, got %v", opts.FileExtensions[FileTypeServerJS])
	}
	if !opts.SkipSubdirectories {
		t.Fatalf("expected SkipSubdirectories to be true")
	}
}

func TestFileExtensionJSAlwaysIncludesTS(t *testing.T) {
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		Extra: map[string]json.RawMessage{
			"fileExtension": json.RawMessage(`"js"`),
		},
	}
	opts, err := OptionsFromConfig("/tmp/project", cfg, nil)
	if err != nil {
		t.Fatalf("OptionsFromConfig failed: %v", err)
	}
	exts := opts.FileExtensions[FileTypeServerJS]
	hasJS := false
	hasTS := false
	for _, ext := range exts {
		if ext == ".js" {
			hasJS = true
		}
		if ext == ".ts" {
			hasTS = true
		}
	}
	if !hasJS {
		t.Fatalf("expected .js in script extensions, got %v", exts)
	}
	if !hasTS {
		t.Fatalf("expected .ts in script extensions (for auto-transpile), got %v", exts)
	}
}

func TestCollectLocalFilesMappingAndIgnore(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(contentDir, "ui"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(contentDir, "skip"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "ui", "page.html"), []byte("<p>hi</p>"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "ignore.txt"), []byte("ignore"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "skip", "Skip.gs"), []byte("function skip() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claspignore"), []byte("src/skip/**\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	ignore, err := config.NewClaspIgnore(root)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed: %v", err)
	}
	opts := Options{
		ProjectRoot:    root,
		RootDir:        "src",
		Ignore:         ignore,
		FileExtensions: DefaultFileExtensions(),
	}
	files, err := CollectLocalFiles(opts)
	if err != nil {
		t.Fatalf("CollectLocalFiles failed: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	byRemote := make(map[string]ProjectFile, len(files))
	for _, file := range files {
		byRemote[file.RemotePath] = file
	}
	if file, ok := byRemote["Code"]; !ok || file.Type != FileTypeServerJS {
		t.Fatalf("expected Code SERVER_JS, got %#v", file)
	}
	if file, ok := byRemote["ui/page"]; !ok || file.Type != FileTypeHTML {
		t.Fatalf("expected ui/page HTML, got %#v", file)
	}
	if file, ok := byRemote["appsscript"]; !ok || file.Type != FileTypeJSON || file.LocalPath != "src/appsscript.json" {
		t.Fatalf("expected appsscript JSON under rootDir, got %#v", file)
	}
}

func TestCollectLocalFilesIncludesLegacyRootAppsscriptFallback(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "src")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	opts := Options{
		ProjectRoot:    root,
		RootDir:        "src",
		FileExtensions: DefaultFileExtensions(),
	}
	files, err := CollectLocalFiles(opts)
	if err != nil {
		t.Fatalf("CollectLocalFiles failed: %v", err)
	}
	byRemote := make(map[string]ProjectFile, len(files))
	for _, file := range files {
		byRemote[file.RemotePath] = file
	}
	if file, ok := byRemote["appsscript"]; !ok || file.Type != FileTypeJSON || file.LocalPath != "appsscript.json" {
		t.Fatalf("expected legacy root appsscript fallback, got %#v", file)
	}
}

func TestCollectLocalFilesSkipsDuplicateRootAppsscript(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	opts := Options{
		ProjectRoot:    root,
		RootDir:        ".",
		FileExtensions: DefaultFileExtensions(),
	}
	files, err := CollectLocalFiles(opts)
	if err != nil {
		t.Fatalf("CollectLocalFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	byRemote := make(map[string]ProjectFile, len(files))
	for _, file := range files {
		byRemote[file.RemotePath] = file
	}
	if file, ok := byRemote["Code"]; !ok || file.Type != FileTypeServerJS {
		t.Fatalf("expected Code SERVER_JS, got %#v", file)
	}
	if file, ok := byRemote["appsscript"]; !ok || file.Type != FileTypeJSON {
		t.Fatalf("expected appsscript JSON, got %#v", file)
	}
}

func TestCollectLocalFilesRejectsRemoteNameConflicts(t *testing.T) {
	t.Run("across-types", func(t *testing.T) {
		root := t.TempDir()
		contentDir := filepath.Join(root, "src")
		if err := os.MkdirAll(contentDir, 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(contentDir, "Code.gs"), []byte("function a() {}"), 0644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(contentDir, "Code.html"), []byte("<p>hi</p>"), 0644); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		opts := Options{
			ProjectRoot:    root,
			RootDir:        "src",
			FileExtensions: DefaultFileExtensions(),
		}
		if _, err := CollectLocalFiles(opts); err == nil {
			t.Fatalf("expected conflict error, got nil")
		}
	})

	t.Run("same-type-extensions", func(t *testing.T) {
		root := t.TempDir()
		contentDir := filepath.Join(root, "src")
		if err := os.MkdirAll(contentDir, 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(contentDir, "page.html"), []byte("<p>html</p>"), 0644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(contentDir, "page.htm"), []byte("<p>htm</p>"), 0644); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		opts := Options{
			ProjectRoot: root,
			RootDir:     "src",
			FileExtensions: map[string][]string{
				FileTypeServerJS: {".gs"},
				FileTypeHTML:     {".html", ".htm"},
				FileTypeJSON:     {".json"},
			},
		}
		if _, err := CollectLocalFiles(opts); err == nil {
			t.Fatalf("expected conflict error, got nil")
		}
	})
}

func TestNormalizeFileExtensionsFallsBackToDefaults(t *testing.T) {
	input := map[string][]string{
		FileTypeServerJS: {"", "  "},
		FileTypeHTML:     {"\t"},
	}
	normalized := normalizeFileExtensions(input)
	defaults := DefaultFileExtensions()

	if !reflect.DeepEqual(normalized[FileTypeServerJS], defaults[FileTypeServerJS]) {
		t.Fatalf("expected server defaults %v, got %v", defaults[FileTypeServerJS], normalized[FileTypeServerJS])
	}
	if !reflect.DeepEqual(normalized[FileTypeHTML], defaults[FileTypeHTML]) {
		t.Fatalf("expected html defaults %v, got %v", defaults[FileTypeHTML], normalized[FileTypeHTML])
	}
	if !reflect.DeepEqual(normalized[FileTypeJSON], defaults[FileTypeJSON]) {
		t.Fatalf("expected json defaults %v, got %v", defaults[FileTypeJSON], normalized[FileTypeJSON])
	}
}

func TestApplyRemoteContentWritesFiles(t *testing.T) {
	root := t.TempDir()
	opts := Options{
		ProjectRoot: root,
		RootDir:     "src",
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".ts"},
			FileTypeHTML:     {".html"},
			FileTypeJSON:     {".json"},
		},
	}
	content := &script.Content{
		Files: []*script.File{
			{
				Name:   "Code",
				Type:   FileTypeServerJS,
				Source: "function a() {}",
			},
			{
				Name:   "ui/page",
				Type:   FileTypeHTML,
				Source: "<p>hi</p>",
			},
			{
				Name:   "appsscript",
				Type:   FileTypeJSON,
				Source: "{}",
			},
		},
	}

	written, err := ApplyRemoteContent(opts, content)
	if err != nil {
		t.Fatalf("ApplyRemoteContent failed: %v", err)
	}
	if len(written) != 3 {
		t.Fatalf("expected 3 files written, got %d", len(written))
	}

	assertFile := func(rel string, expected string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if string(data) != expected {
			t.Fatalf("content mismatch for %s: %s", rel, string(data))
		}
	}
	assertFile("src/Code.ts", "function a() {}")
	assertFile("src/ui/page.html", "<p>hi</p>")
	assertFile("src/appsscript.json", "{}")
}

func TestArchiveLocalFilesWritesFiles(t *testing.T) {
	archiveRoot := t.TempDir()
	files := []ProjectFile{
		{
			LocalPath: "src/Code.gs",
			Source:    "function a() {}",
			Type:      FileTypeServerJS,
		},
		{
			LocalPath: "appsscript.json",
			Source:    "{}",
			Type:      FileTypeJSON,
		},
	}

	if err := ArchiveLocalFiles(archiveRoot, files); err != nil {
		t.Fatalf("ArchiveLocalFiles failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveRoot, "src", "Code.gs")); err != nil {
		t.Fatalf("expected archived Code.gs to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveRoot, "appsscript.json")); err != nil {
		t.Fatalf("expected archived appsscript.json to exist: %v", err)
	}
}

func TestArchiveLocalFilesRejectsTraversal(t *testing.T) {
	cases := []string{
		"../evil.gs",
		"/etc/passwd",
		`C:\secret`,
		`\\server\share\file`,
		"dir/../file",
		"bad\x00name",
	}
	for _, localPath := range cases {
		t.Run(localPath, func(t *testing.T) {
			archiveRoot := t.TempDir()
			files := []ProjectFile{
				{
					LocalPath: localPath,
					Source:    "function evil() {}",
					Type:      FileTypeServerJS,
				},
			}

			if err := ArchiveLocalFiles(archiveRoot, files); err == nil {
				t.Fatal("expected error for path traversal")
			}
		})
	}
}

func TestApplyRemoteContentRejectsInvalidRemoteNames(t *testing.T) {
	root := t.TempDir()
	opts := Options{
		ProjectRoot: root,
		RootDir:     "src",
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
			FileTypeHTML:     {".html"},
			FileTypeJSON:     {".json"},
		},
	}
	cases := []string{
		"/etc/passwd",
		`C:\secret`,
		`\\server\share\file`,
		"dir/../file",
		"bad\x00name",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			content := &script.Content{
				Files: []*script.File{
					{
						Name:   name,
						Type:   FileTypeServerJS,
						Source: "function a() {}",
					},
				},
			}
			if _, err := ApplyRemoteContent(opts, content); err == nil {
				t.Fatalf("expected error for remote name %q", name)
			}
		})
	}
}

func TestApplyRemoteContentRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	opts := Options{
		ProjectRoot: root,
		RootDir:     "src",
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
			FileTypeHTML:     {".html"},
			FileTypeJSON:     {".json"},
		},
	}
	content := &script.Content{
		Files: []*script.File{
			{
				Name:   "../../../etc/passwd",
				Type:   FileTypeServerJS,
				Source: "function a() {}",
			},
		},
	}

	if _, err := ApplyRemoteContent(opts, content); err == nil {
		t.Fatalf("expected path traversal error, got nil")
	}
}

func TestContentDirRejectsRootDirPathTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../outside",
		"../../etc",
		"a/../../b",
	}
	for _, rootDir := range cases {
		t.Run(rootDir, func(t *testing.T) {
			opts := Options{ProjectRoot: root, RootDir: rootDir}
			if _, err := CollectLocalFiles(opts); err == nil {
				t.Fatalf("expected error for rootDir %q, got nil", rootDir)
			}
		})
	}
}

// A rootDir that is lexically inside the project but reaches outside through
// a symlinked intermediate component must be rejected in both directions,
// with or without AllowSymlinks — the escape is the rootDir itself, not an
// individual file inside it.
func TestContentDirRejectsRootDirEscapingViaIntermediateSymlink(t *testing.T) {
	for _, allowSymlinks := range []bool{false, true} {
		name := "allow-symlinks=false"
		if allowSymlinks {
			name = "allow-symlinks=true"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			if err := os.MkdirAll(filepath.Join(outside, "src"), 0755); err != nil {
				t.Fatalf("failed to create outside dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outside, "src", "Secret.gs"), []byte("secret"), 0644); err != nil {
				t.Fatalf("failed to write outside file: %v", err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
				t.Skipf("symlinks not supported on this platform: %v", err)
			}

			opts := Options{
				ProjectRoot:   root,
				RootDir:       "link/src",
				AllowSymlinks: allowSymlinks,
				FileExtensions: map[string][]string{
					FileTypeServerJS: {".gs"},
				},
			}
			if files, err := CollectLocalFiles(opts); err == nil {
				t.Fatalf("expected CollectLocalFiles to reject escaping rootDir, collected %+v", files)
			}
			content := &script.Content{
				Files: []*script.File{
					{Name: "Written", Type: FileTypeServerJS, Source: "function a() {}"},
				},
			}
			if _, err := ApplyRemoteContent(opts, content); err == nil {
				t.Fatal("expected ApplyRemoteContent to reject escaping rootDir")
			}
			if _, err := os.Stat(filepath.Join(outside, "src", "Written.gs")); !os.IsNotExist(err) {
				t.Fatalf("expected no file written outside the project, stat err = %v", err)
			}
		})
	}
}

// create/clone resolve the content directory before it exists on disk, so a
// not-yet-created rootDir must still be accepted.
func TestContentDirAllowsNotYetCreatedRootDir(t *testing.T) {
	root := t.TempDir()
	opts := Options{
		ProjectRoot: root,
		RootDir:     "src/nested",
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
	}
	content := &script.Content{
		Files: []*script.File{
			{Name: "Code", Type: FileTypeServerJS, Source: "function a() {}"},
		},
	}
	if _, err := ApplyRemoteContent(opts, content); err != nil {
		t.Fatalf("expected not-yet-created rootDir to be accepted, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "nested", "Code.gs")); err != nil {
		t.Fatalf("expected Code.gs to be written: %v", err)
	}
}

func TestContentDirAllowsRootDirWithinProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	opts := Options{ProjectRoot: root, RootDir: "./src"}
	if _, err := CollectLocalFiles(opts); err != nil {
		t.Fatalf("expected no error for valid rootDir, got %v", err)
	}
}

func TestCollectLocalFilesSkipsSymlinksByDefault(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	contentDir := filepath.Join(root, "src")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("failed to create content dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("failed to write Code.gs: %v", err)
	}
	secretPath := filepath.Join(outside, "secret.gs")
	if err := os.WriteFile(secretPath, []byte("function secret() {}"), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}
	linkPath := filepath.Join(contentDir, "Linked.gs")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	opts := Options{
		ProjectRoot: root,
		RootDir:     "src",
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
	}
	files, err := CollectLocalFiles(opts)
	if err != nil {
		t.Fatalf("CollectLocalFiles failed: %v", err)
	}
	for _, f := range files {
		if f.LocalPath == "src/Linked.gs" {
			t.Fatalf("expected symlinked file to be skipped by default, got %+v", f)
		}
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (Code.gs only), got %d: %+v", len(files), files)
	}
}

func TestCollectLocalFilesAllowSymlinksRejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	contentDir := filepath.Join(root, "src")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("failed to create content dir: %v", err)
	}
	secretPath := filepath.Join(outside, "secret.gs")
	if err := os.WriteFile(secretPath, []byte("function secret() {}"), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}
	linkPath := filepath.Join(contentDir, "Linked.gs")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	opts := Options{
		ProjectRoot:   root,
		RootDir:       "src",
		AllowSymlinks: true,
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
	}
	if _, err := CollectLocalFiles(opts); err == nil {
		t.Fatalf("expected error for symlink escaping rootDir even with AllowSymlinks, got nil")
	}
}

func TestCollectLocalFilesAllowSymlinksFollowsWithinRoot(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "src")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("failed to create content dir: %v", err)
	}
	targetPath := filepath.Join(contentDir, "Target.gs")
	if err := os.WriteFile(targetPath, []byte("function target() {}"), 0644); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}
	linkPath := filepath.Join(contentDir, "Linked.gs")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	opts := Options{
		ProjectRoot:   root,
		RootDir:       "src",
		AllowSymlinks: true,
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
	}
	files, err := CollectLocalFiles(opts)
	if err != nil {
		t.Fatalf("CollectLocalFiles failed: %v", err)
	}
	found := false
	for _, f := range files {
		if f.LocalPath == "src/Linked.gs" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected src/Linked.gs to be collected with AllowSymlinks, got %+v", files)
	}
}

// When rootDir itself is a symlinked directory (pointing at another
// directory inside the project, so contentDir()'s containment check does
// not reject it outright), filepath.WalkDir visits that root entry once but
// — unlike a symlink found inside contentDir — does not descend into it
// even when AllowSymlinks is true (WalkDir never follows a symlinked
// directory's contents; the "follow" special-case only applies to the
// walk's own starting argument, and even then only as a single visited
// entry, not a traversal). So no files are ever collected from a symlinked
// rootDir, regardless of AllowSymlinks, and the skip is reported once for
// the root itself with an accurate "directory" message.
//
// (A rootDir symlink resolving *outside* the project is instead rejected
// earlier, by contentDir()'s own containment check — see
// TestContentDirRejectsRootDirEscapingViaIntermediateSymlink.)
func TestCollectLocalFilesSkipsSymlinkedRootDirectory(t *testing.T) {
	for _, allowSymlinks := range []bool{false, true} {
		name := "allow-symlinks=false"
		if allowSymlinks {
			name = "allow-symlinks=true"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			actualDir := filepath.Join(root, "actual")
			if err := os.MkdirAll(actualDir, 0755); err != nil {
				t.Fatalf("failed to create actual dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(actualDir, "Code.gs"), []byte("function a() {}"), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
			linkPath := filepath.Join(root, "src")
			if err := os.Symlink(actualDir, linkPath); err != nil {
				t.Skipf("symlinks not supported on this platform: %v", err)
			}

			var skipped []SkippedFile
			opts := Options{
				ProjectRoot:   root,
				RootDir:       "src",
				AllowSymlinks: allowSymlinks,
				FileExtensions: map[string][]string{
					FileTypeServerJS: {".gs"},
				},
				OnSkip: func(s SkippedFile) {
					skipped = append(skipped, s)
				},
			}
			files, err := CollectLocalFiles(opts)
			if err != nil {
				t.Fatalf("CollectLocalFiles failed: %v", err)
			}
			if len(files) != 0 {
				t.Fatalf("expected no files collected from a symlinked rootDir, got %+v", files)
			}
			if len(skipped) != 1 || skipped[0].Reason != SkipReasonSymlink {
				t.Fatalf("expected exactly 1 symlink skip for the root itself, got %+v", skipped)
			}
		})
	}
}

// A symlink that resolves inside rootDir but under .glasp/ must be rejected
// even with AllowSymlinks: .glasp/ holds internal data (tokens, archives,
// config) that must never be collected, regardless of how it's reached.
func TestCollectLocalFilesRejectsSymlinkIntoGlaspDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".glasp"), 0755); err != nil {
		t.Fatalf("failed to create .glasp dir: %v", err)
	}
	secretPath := filepath.Join(root, ".glasp", "access.json")
	if err := os.WriteFile(secretPath, []byte(`{"token":"SECRET_TOKEN"}`), 0600); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}
	linkPath := filepath.Join(root, "Code.gs")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	opts := Options{
		ProjectRoot:   root,
		RootDir:       ".",
		AllowSymlinks: true,
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
	}
	files, err := CollectLocalFiles(opts)
	if err == nil {
		for _, f := range files {
			if f.Source == `{"token":"SECRET_TOKEN"}` {
				t.Fatalf("token file content was collected as %q", f.LocalPath)
			}
		}
		t.Fatal("expected an error rejecting a symlink into .glasp/, got nil")
	}
}

// A remote file whose local target directory is already a symlink pointing
// outside the project (left behind by a cloned repository, or a prior
// state) must not be written through, even though the remote name itself is
// lexically clean.
func TestApplyRemoteContentRejectsExistingParentSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "src", "pkg")); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	opts := Options{
		ProjectRoot: root,
		RootDir:     "src",
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
	}
	content := &script.Content{
		Files: []*script.File{
			{Name: "pkg/Code", Type: FileTypeServerJS, Source: "function a(){}"},
		},
	}
	if _, err := ApplyRemoteContent(opts, content); err == nil {
		t.Fatal("expected an error rejecting a write through an existing parent symlink")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "Code.gs")); statErr == nil {
		t.Fatalf("file was written outside the project at %s", filepath.Join(outside, "Code.gs"))
	}
}

// An ignored path that was never going to be collected anyway (its
// extension doesn't match any configured type) must not be reported via
// OnSkip — only files that would otherwise have been candidates count
// toward "Skipped N file(s)".
func TestCollectLocalFilesDoesNotReportIgnoredNonCandidates(t *testing.T) {
	root := t.TempDir()
	nodeModulesDir := filepath.Join(root, "node_modules", "lib")
	if err := os.MkdirAll(nodeModulesDir, 0755); err != nil {
		t.Fatalf("failed to create node_modules dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeModulesDir, "README.md"), []byte("# readme"), 0644); err != nil {
		t.Fatalf("failed to write README.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Code.gs"), []byte("function a(){}"), 0644); err != nil {
		t.Fatalf("failed to write Code.gs: %v", err)
	}
	// node_modules/ is ignored by a built-in default pattern even without a
	// .claspignore file.
	ignore, err := config.NewClaspIgnore(root)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed: %v", err)
	}

	var skipped []SkippedFile
	opts := Options{
		ProjectRoot: root,
		RootDir:     ".",
		Ignore:      ignore,
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
		OnSkip: func(s SkippedFile) { skipped = append(skipped, s) },
	}
	if _, err := CollectLocalFiles(opts); err != nil {
		t.Fatalf("CollectLocalFiles failed: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected no OnSkip reports for a non-candidate ignored file, got %+v", skipped)
	}
}

// Per-file skip detail must only be visible at --log-level debug, matching
// usage.md's documented contract ("Per-file details are available with
// --log-level debug"); the one-line "Skipped N file(s)" summary is what's
// shown by default.
func TestCollectLocalFilesLogsSymlinkSkipAtDebugLevel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Target.gs"), []byte("function a(){}"), 0644); err != nil {
		t.Fatalf("failed to write target: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "Target.gs"), filepath.Join(root, "Linked.gs")); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}
	opts := Options{
		ProjectRoot: root,
		RootDir:     ".",
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs"},
		},
	}

	// At the default (info) level, nothing about the skipped symlink should
	// be logged.
	infoLog := func() string {
		var buf bytes.Buffer
		orig := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
		defer slog.SetDefault(orig)
		if _, err := CollectLocalFiles(opts); err != nil {
			t.Fatalf("CollectLocalFiles failed: %v", err)
		}
		return buf.String()
	}()
	if infoLog != "" {
		t.Fatalf("expected no log output at info level, got: %q", infoLog)
	}

	// At debug level, the detail is available.
	debugLog := captureLog(t, func() {
		if _, err := CollectLocalFiles(opts); err != nil {
			t.Fatalf("CollectLocalFiles failed: %v", err)
		}
	})
	if debugLog == "" {
		t.Fatal("expected symlink skip detail to be logged at debug level")
	}
}

func TestCollectLocalFilesReportsSkippedFiles(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "src")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatalf("failed to create content dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("failed to write Code.gs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "types.d.ts"), []byte("declare const x: number;"), 0644); err != nil {
		t.Fatalf("failed to write types.d.ts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "ignored.gs"), []byte("function b() {}"), 0644); err != nil {
		t.Fatalf("failed to write ignored.gs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claspignore"), []byte("src/ignored.gs\n"), 0644); err != nil {
		t.Fatalf("failed to write .claspignore: %v", err)
	}
	ignore, err := config.NewClaspIgnore(root)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed: %v", err)
	}

	var skipped []SkippedFile
	opts := Options{
		ProjectRoot: root,
		RootDir:     "src",
		Ignore:      ignore,
		FileExtensions: map[string][]string{
			FileTypeServerJS: {".gs", ".ts"},
		},
		OnSkip: func(s SkippedFile) {
			skipped = append(skipped, s)
		},
	}
	if _, err := CollectLocalFiles(opts); err != nil {
		t.Fatalf("CollectLocalFiles failed: %v", err)
	}

	byReason := map[SkipReason]int{}
	for _, s := range skipped {
		byReason[s.Reason]++
	}
	if byReason[SkipReasonIgnored] != 1 {
		t.Fatalf("expected 1 ignored skip, got %d (%+v)", byReason[SkipReasonIgnored], skipped)
	}
	if byReason[SkipReasonDeclaration] != 1 {
		t.Fatalf("expected 1 declaration skip, got %d (%+v)", byReason[SkipReasonDeclaration], skipped)
	}
}

func TestSortFilesByPushOrder(t *testing.T) {
	t.Run("orders-matching-prefix", func(t *testing.T) {
		files := []ProjectFile{
			{LocalPath: "src/C.gs"},
			{LocalPath: "src/B.gs"},
			{LocalPath: "src/A.gs"},
		}
		order := []string{"src/B.gs", "src/A.gs"}

		SortFilesByPushOrder(files, order, "src")

		if files[0].LocalPath != "src/B.gs" || files[1].LocalPath != "src/A.gs" {
			t.Fatalf("unexpected order: %#v", []string{files[0].LocalPath, files[1].LocalPath, files[2].LocalPath})
		}
	})

	t.Run("orders-with-root-prefix", func(t *testing.T) {
		files := []ProjectFile{
			{LocalPath: "src/B.gs"},
			{LocalPath: "src/A.gs"},
			{LocalPath: "src/C.gs"},
		}
		order := []string{"A.gs", "C.gs"}

		SortFilesByPushOrder(files, order, "src")

		if files[0].LocalPath != "src/A.gs" || files[1].LocalPath != "src/C.gs" {
			t.Fatalf("unexpected order: %#v", []string{files[0].LocalPath, files[1].LocalPath, files[2].LocalPath})
		}
	})

	t.Run("no-order-keeps-lexicographic", func(t *testing.T) {
		files := []ProjectFile{
			{LocalPath: "src/B.gs"},
			{LocalPath: "src/A.gs"},
		}

		SortFilesByPushOrder(files, nil, "src")

		if files[0].LocalPath != "src/B.gs" || files[1].LocalPath != "src/A.gs" {
			t.Fatalf("unexpected order: %#v", []string{files[0].LocalPath, files[1].LocalPath})
		}
	})

	t.Run("orders-known-before-unknown", func(t *testing.T) {
		files := []ProjectFile{
			{LocalPath: "src/C.gs"},
			{LocalPath: "src/B.gs"},
			{LocalPath: "src/A.gs"},
		}
		order := []string{"src/B.gs"}

		SortFilesByPushOrder(files, order, "src")

		if files[0].LocalPath != "src/B.gs" {
			t.Fatalf("unexpected order: %#v", []string{files[0].LocalPath, files[1].LocalPath, files[2].LocalPath})
		}
	})
}

func TestBuildContent(t *testing.T) {
	files := []ProjectFile{
		{RemotePath: "Code", Type: FileTypeServerJS, Source: "function a() {}"},
		{RemotePath: "ui/page", Type: FileTypeHTML, Source: "<p>hi</p>"},
	}

	content := BuildContent(files)

	if content == nil || len(content.Files) != 2 {
		t.Fatalf("unexpected content: %#v", content)
	}
	if content.Files[0].Name != "Code" || content.Files[0].Type != FileTypeServerJS || content.Files[0].Source != "function a() {}" {
		t.Fatalf("unexpected file[0]: %#v", content.Files[0])
	}
	if content.Files[1].Name != "ui/page" || content.Files[1].Type != FileTypeHTML || content.Files[1].Source != "<p>hi</p>" {
		t.Fatalf("unexpected file[1]: %#v", content.Files[1])
	}
}

func TestValidateRelPath(t *testing.T) {
	valid := []string{
		"Code.gs",
		"src/Code.gs",
		"a/b/c.html",
		"appsscript.json",
		"dir.with.dots/file.js",
		"..name/file.js", // ".." only as a full path element is rejected
	}
	for _, p := range valid {
		if !validateRelPath(p) {
			t.Errorf("validateRelPath(%q) = false, want true", p)
		}
	}
	invalid := []string{
		"",
		"a\x00b",
		"/etc/passwd",
		"../escape.js",
		"a/../../escape.js",
		"src/..",
		`\\server\share\file.js`,
		"C:evil.js",
		"c:/evil.js",
	}
	for _, p := range invalid {
		if validateRelPath(p) {
			t.Errorf("validateRelPath(%q) = true, want false", p)
		}
	}
}
