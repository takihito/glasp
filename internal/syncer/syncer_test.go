package syncer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"glasp/internal/config"
	"google.golang.org/api/script/v1"
)

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
	if opts.FileExtensions[fileTypeServerJS][0] != ".ts" {
		t.Fatalf("expected .ts, got %v", opts.FileExtensions[fileTypeServerJS])
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
	exts := opts.FileExtensions[fileTypeServerJS]
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
	if file, ok := byRemote["Code"]; !ok || file.Type != fileTypeServerJS {
		t.Fatalf("expected Code SERVER_JS, got %#v", file)
	}
	if file, ok := byRemote["ui/page"]; !ok || file.Type != fileTypeHTML {
		t.Fatalf("expected ui/page HTML, got %#v", file)
	}
	if file, ok := byRemote["appsscript"]; !ok || file.Type != fileTypeJSON || file.LocalPath != "src/appsscript.json" {
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
	if file, ok := byRemote["appsscript"]; !ok || file.Type != fileTypeJSON || file.LocalPath != "appsscript.json" {
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
	if file, ok := byRemote["Code"]; !ok || file.Type != fileTypeServerJS {
		t.Fatalf("expected Code SERVER_JS, got %#v", file)
	}
	if file, ok := byRemote["appsscript"]; !ok || file.Type != fileTypeJSON {
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
				fileTypeServerJS: {".gs"},
				fileTypeHTML:     {".html", ".htm"},
				fileTypeJSON:     {".json"},
			},
		}
		if _, err := CollectLocalFiles(opts); err == nil {
			t.Fatalf("expected conflict error, got nil")
		}
	})
}

func TestNormalizeFileExtensionsFallsBackToDefaults(t *testing.T) {
	input := map[string][]string{
		fileTypeServerJS: {"", "  "},
		fileTypeHTML:     {"\t"},
	}
	normalized := normalizeFileExtensions(input)
	defaults := DefaultFileExtensions()

	if !reflect.DeepEqual(normalized[fileTypeServerJS], defaults[fileTypeServerJS]) {
		t.Fatalf("expected server defaults %v, got %v", defaults[fileTypeServerJS], normalized[fileTypeServerJS])
	}
	if !reflect.DeepEqual(normalized[fileTypeHTML], defaults[fileTypeHTML]) {
		t.Fatalf("expected html defaults %v, got %v", defaults[fileTypeHTML], normalized[fileTypeHTML])
	}
	if !reflect.DeepEqual(normalized[fileTypeJSON], defaults[fileTypeJSON]) {
		t.Fatalf("expected json defaults %v, got %v", defaults[fileTypeJSON], normalized[fileTypeJSON])
	}
}

func TestApplyRemoteContentWritesFiles(t *testing.T) {
	root := t.TempDir()
	opts := Options{
		ProjectRoot: root,
		RootDir:     "src",
		FileExtensions: map[string][]string{
			fileTypeServerJS: {".ts"},
			fileTypeHTML:     {".html"},
			fileTypeJSON:     {".json"},
		},
	}
	content := &script.Content{
		Files: []*script.File{
			{
				Name:   "Code",
				Type:   fileTypeServerJS,
				Source: "function a() {}",
			},
			{
				Name:   "ui/page",
				Type:   fileTypeHTML,
				Source: "<p>hi</p>",
			},
			{
				Name:   "appsscript",
				Type:   fileTypeJSON,
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
			Type:      fileTypeServerJS,
		},
		{
			LocalPath: "appsscript.json",
			Source:    "{}",
			Type:      fileTypeJSON,
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
					Type:      fileTypeServerJS,
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
			fileTypeServerJS: {".gs"},
			fileTypeHTML:     {".html"},
			fileTypeJSON:     {".json"},
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
						Type:   fileTypeServerJS,
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
			fileTypeServerJS: {".gs"},
			fileTypeHTML:     {".html"},
			fileTypeJSON:     {".json"},
		},
	}
	content := &script.Content{
		Files: []*script.File{
			{
				Name:   "../../../etc/passwd",
				Type:   fileTypeServerJS,
				Source: "function a() {}",
			},
		},
	}

	if _, err := ApplyRemoteContent(opts, content); err == nil {
		t.Fatalf("expected path traversal error, got nil")
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
		{RemotePath: "Code", Type: fileTypeServerJS, Source: "function a() {}"},
		{RemotePath: "ui/page", Type: fileTypeHTML, Source: "<p>hi</p>"},
	}

	content := BuildContent(files)

	if content == nil || len(content.Files) != 2 {
		t.Fatalf("unexpected content: %#v", content)
	}
	if content.Files[0].Name != "Code" || content.Files[0].Type != fileTypeServerJS || content.Files[0].Source != "function a() {}" {
		t.Fatalf("unexpected file[0]: %#v", content.Files[0])
	}
	if content.Files[1].Name != "ui/page" || content.Files[1].Type != fileTypeHTML || content.Files[1].Source != "<p>hi</p>" {
		t.Fatalf("unexpected file[1]: %#v", content.Files[1])
	}
}
