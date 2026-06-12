package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/history"
	"github.com/takihito/glasp/internal/syncer"
	"google.golang.org/api/script/v1"
)

func TestPushCommandArchiveFlag(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Archive: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
	if !entries[0].IsDir() {
		t.Fatalf("expected archive entry to be a directory")
	}
	archiveDir := filepath.Join(archiveBase, entries[0].Name())
	if _, err := os.Stat(filepath.Join(archiveDir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json to exist: %v", err)
	}
	manifestData, err := os.ReadFile(filepath.Join(archiveDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	var manifest archiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}
	if manifest.Status != "success" {
		t.Fatalf("expected manifest status success, got %s", manifest.Status)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "working", "src", "Code.gs")); err != nil {
		t.Fatalf("expected working Code.gs to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "payload", "src", "Code.gs")); err != nil {
		t.Fatalf("expected payload Code.gs to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "canonical")); !os.IsNotExist(err) {
		t.Fatalf("expected canonical directory to be absent on push")
	}
}

func TestPushCommandArchiveConfig(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := config.SaveGlaspConfig(root, &config.GlaspConfig{Archive: config.ArchiveConfig{Push: true}}); err != nil {
		t.Fatalf("SaveGlaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.gs"), []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Archive: false}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
}

func TestPushCommandArchiveWithTypeScriptConversion(t *testing.T) {
	root := useTempDir(t)
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		RootDir:  "src",
		Extra: map[string]json.RawMessage{
			"fileExtension": json.RawMessage(`"ts"`),
		},
	}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.ts"), []byte("const msg: string = 'hi'"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Archive: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	file := findContentFile(fake.updateContent, "Code")
	if file == nil {
		t.Fatalf("expected pushed Code file")
	}
	if strings.Contains(file.Source, ": string") {
		t.Fatalf("expected TypeScript annotation to be removed from payload, got: %s", file.Source)
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
	entries, err := os.ReadDir(archiveBase)
	if err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
	archiveDir := filepath.Join(archiveBase, entries[0].Name())
	if _, err := os.Stat(filepath.Join(archiveDir, "working", "src", "Code.ts")); err != nil {
		t.Fatalf("expected working Code.ts to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "payload", "src", "Code.js")); err != nil {
		t.Fatalf("expected payload Code.js to exist: %v", err)
	}
}

func TestPushCommandAutoTranspileTSWithoutFileExtension(t *testing.T) {
	root := useTempDir(t)
	// No fileExtension set in .clasp.json
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		RootDir:  "src",
	}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Code.ts"), []byte("const msg: string = 'hi'"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Util.js"), []byte("function util() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	fake := &fakeScriptClient{updateContentResp: &script.Content{}}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}

	// .ts file should be transpiled (type annotation removed).
	codeFile := findContentFile(fake.updateContent, "Code")
	if codeFile == nil {
		t.Fatalf("expected pushed Code file")
	}
	if strings.Contains(codeFile.Source, ": string") {
		t.Fatalf("expected TypeScript annotation to be removed, got: %s", codeFile.Source)
	}

	// .js file should be passed through unchanged.
	utilFile := findContentFile(fake.updateContent, "Util")
	if utilFile == nil {
		t.Fatalf("expected pushed Util file")
	}
	if utilFile.Source != "function util() {}" {
		t.Fatalf("expected .js file to be unchanged, got: %s", utilFile.Source)
	}
}

func TestArchivePushRunWritesFailedManifestOnError(t *testing.T) {
	root := useTempDir(t)
	working := []syncer.ProjectFile{
		{
			LocalPath: "src/Code.gs",
			Type:      "SERVER_JS",
			Source:    "function a() {}",
		},
	}
	payload := []syncer.ProjectFile{
		{
			LocalPath: "../evil.gs",
			Type:      "SERVER_JS",
			Source:    "function evil() {}",
		},
	}

	_, err := archivePushRun(root, "script-id", working, payload, "gs", "")
	if err == nil {
		t.Fatalf("expected archivePushRun to fail")
	}

	archiveBase := filepath.Join(root, ".glasp", "archive", "script-id", "push")
	entries, readErr := os.ReadDir(archiveBase)
	if readErr != nil {
		t.Fatalf("expected archive dir to exist: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive directory, got %d", len(entries))
	}
	manifestData, readErr := os.ReadFile(filepath.Join(archiveBase, entries[0].Name(), "manifest.json"))
	if readErr != nil {
		t.Fatalf("expected failed manifest to exist: %v", readErr)
	}
	var manifest archiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}
	if manifest.Status != "failed" {
		t.Fatalf("expected manifest status failed, got %s", manifest.Status)
	}
}
func TestPushCommandFlow(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
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

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{Force: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if !fake.updateContentCalled {
		t.Fatal("expected UpdateContent to be called")
	}
	if fake.updateScriptID != "script-id" {
		t.Fatalf("expected script-id, got %s", fake.updateScriptID)
	}
	if file := findContentFile(fake.updateContent, "Code"); file == nil || file.Type != "SERVER_JS" {
		t.Fatalf("expected Code SERVER_JS, got %#v", file)
	}
	if file := findContentFile(fake.updateContent, "appsscript"); file == nil || file.Type != "JSON" {
		t.Fatalf("expected appsscript JSON, got %#v", file)
	}
}

func TestPushCommandPassesAuthPath(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
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

	fake := &fakeScriptClient{}
	var gotAuthPath string
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		gotAuthPath = authPath
		return fake, nil
	}

	if err := (&PushCmd{Force: true, Auth: " ./auth/.clasprc.json "}).Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if gotAuthPath != filepath.Clean("./auth/.clasprc.json") {
		t.Fatalf("expected cleaned auth path, got %q", gotAuthPath)
	}
}

func TestPushCommandDryRunSkipsAPIAndArchive(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
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

	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected dryrun push to skip client creation")
		return nil, nil
	}

	cmd := PushCmd{Force: true, Archive: true, DryRun: true}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".glasp", "archive")); !os.IsNotExist(err) {
		t.Fatalf("expected archive not to be created during dryrun")
	}
}

func TestPushCLIParsesHistoryID(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"push", "--history-id", "42"})
	if err != nil {
		t.Fatalf("expected push --history-id to parse, got %v", err)
	}
	if cli.Push.HistoryID != 42 {
		t.Fatalf("expected history-id 42, got %d", cli.Push.HistoryID)
	}
}

func TestPushCommandHistoryIDUsesArchivePayload(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120000")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120000","fileExtension":"js","convert":"none","status":"success"}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := PushCmd{HistoryID: 1}
	out, err := captureStdout(t, func() error { return cmd.Run(nil) })
	if err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if !fake.updateContentCalled {
		t.Fatalf("expected UpdateContent to be called")
	}
	if !strings.Contains(out, "Using history source id=1") {
		t.Fatalf("expected history source output, got %q", out)
	}
	if !strings.Contains(out, "manifest.timestamp=20260309_120000") {
		t.Fatalf("expected manifest timestamp output, got %q", out)
	}
	if file := findContentFile(fake.updateContent, "Code"); file == nil || file.Type != "SERVER_JS" {
		t.Fatalf("expected Code SERVER_JS in replay payload, got %#v", file)
	}
}

func TestPushCommandHistoryIDDryRunSkipsAPI(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120001")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120001","fileExtension":"js","convert":"none","status":"success"}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected dryrun history push to skip client creation")
		return nil, nil
	}

	out, err := captureStdout(t, func() error { return (&PushCmd{HistoryID: 1, DryRun: true}).Run(nil) })
	if err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if !strings.Contains(out, "Using history source id=1") {
		t.Fatalf("expected history source output, got %q", out)
	}
	if !strings.Contains(out, "manifest.timestamp=20260309_120001") {
		t.Fatalf("expected manifest timestamp output, got %q", out)
	}
}

func TestPushCommandHistoryIDLegacyArchiveStripsRootDirPrefix(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120002")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload", "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120002","fileExtension":"js","convert":"none","status":"success"}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "src", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	if err := (&PushCmd{HistoryID: 1}).Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if file := findContentFile(fake.updateContent, "Code"); file == nil {
		t.Fatalf("expected remote path Code to be used, got %#v", fake.updateContent)
	}
	if file := findContentFile(fake.updateContent, "src/Code"); file != nil {
		t.Fatalf("expected src/Code not to be used, got %#v", file)
	}
}

func TestPushCommandHistoryIDPrefersManifestPayloadIndexRemotePath(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	archivePath := filepath.Join(root, ".glasp", "archive", "script-id", "push", "20260309_120003")
	if err := os.MkdirAll(filepath.Join(archivePath, "payload", "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "manifest.json"), []byte(`{"scriptId":"script-id","direction":"push","timestamp":"20260309_120003","fileExtension":"js","convert":"none","status":"success","payloadIndex":[{"path":"src/Code.js","remotePath":"SpecialCode","type":"SERVER_JS"}]}`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archivePath, "payload", "src", "Code.js"), []byte("function fromHistory() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	if err := history.Append(root, history.Entry{
		Command: "push",
		Status:  "success",
		Archive: history.Archive{
			Enabled:   true,
			Direction: "push",
			Path:      archivePath,
		},
	}); err != nil {
		t.Fatalf("append history failed: %v", err)
	}

	fake := &fakeScriptClient{}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	if err := (&PushCmd{HistoryID: 1}).Run(nil); err != nil {
		t.Fatalf("PushCmd.Run failed: %v", err)
	}
	if file := findContentFile(fake.updateContent, "SpecialCode"); file == nil {
		t.Fatalf("expected payloadIndex remote path SpecialCode, got %#v", fake.updateContent)
	}
}

func TestPushCommandHistoryIDErrorsWhenMissing(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	err := (&PushCmd{HistoryID: 99}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for missing history id")
	}
	if !strings.Contains(err.Error(), "history id 99 not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPushCommandDryRunRejectsEmptyAuthPath(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}); err != nil {
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
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	err := (&PushCmd{Force: true, DryRun: true, Auth: "   "}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for empty --auth path in dryrun push")
	}
	if !strings.Contains(err.Error(), "--auth path is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPushCommandRejectsTSXFileExtension(t *testing.T) {
	root := useTempDir(t)
	cfg := &config.ClaspConfig{
		ScriptID: "script-id",
		RootDir:  "src",
		Extra: map[string]json.RawMessage{
			"fileExtension": json.RawMessage(`"tsx"`),
		},
	}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	err := (&PushCmd{}).Run(nil)
	if err == nil {
		t.Fatalf("expected error for tsx fileExtension")
	}
	if !strings.Contains(err.Error(), `fileExtension "tsx" is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
