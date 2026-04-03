package transform

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"glasp/internal/config"
	"glasp/internal/syncer"
)

func TestConvertGasToTS(t *testing.T) {
	root := t.TempDir()
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
	if err := os.WriteFile(filepath.Join(root, "src", "page.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	ignore, err := config.NewClaspIgnore(root)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed: %v", err)
	}
	opts, err := syncer.OptionsFromConfig(root, cfg, ignore)
	if err != nil {
		t.Fatalf("OptionsFromConfig failed: %v", err)
	}
	opts.FileExtensions = syncer.DefaultFileExtensions()

	outDir := filepath.Join(root, "out-ts")
	result, err := Convert(opts, outDir, ModeGasToTS, nil)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	if result.OutDir != outDir {
		t.Fatalf("expected out dir %s, got %s", outDir, result.OutDir)
	}
	if _, err := os.Stat(filepath.Join(outDir, "Code.ts")); err != nil {
		t.Fatalf("expected Code.ts to exist: %v", err)
	}
	code, err := os.ReadFile(filepath.Join(outDir, "Code.ts"))
	if err != nil {
		t.Fatalf("read Code.ts failed: %v", err)
	}
	if !strings.Contains(string(code), "function a()") {
		t.Fatalf("expected converted output to include function, got: %s", string(code))
	}
	if _, err := os.Stat(filepath.Join(outDir, "page.html")); err != nil {
		t.Fatalf("expected page.html to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "appsscript.json")); err != nil {
		t.Fatalf("expected appsscript.json to exist: %v", err)
	}
}

func TestConvertTSToGasSkipsDTS(t *testing.T) {
	root := t.TempDir()
	cfg := &config.ClaspConfig{ScriptID: "script-id", RootDir: "src"}
	if err := config.SaveClaspConfig(root, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "Main.ts"), []byte("const msg: string = 'hi'"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "types.d.ts"), []byte("declare const foo: string;"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "appsscript.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	ignore, err := config.NewClaspIgnore(root)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed: %v", err)
	}
	opts, err := syncer.OptionsFromConfig(root, cfg, ignore)
	if err != nil {
		t.Fatalf("OptionsFromConfig failed: %v", err)
	}
	opts.FileExtensions = syncer.DefaultFileExtensions()
	opts.FileExtensions["SERVER_JS"] = []string{".ts"}

	outDir := filepath.Join(root, "out-gas")
	if _, err := Convert(opts, outDir, ModeTSToGas, nil); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "Main.js")); err != nil {
		t.Fatalf("expected Main.js to exist: %v", err)
	}
	code, err := os.ReadFile(filepath.Join(outDir, "Main.js"))
	if err != nil {
		t.Fatalf("read Main.js failed: %v", err)
	}
	if !strings.Contains(string(code), "msg =") {
		t.Fatalf("expected converted output to include msg assignment, got: %s", string(code))
	}
	if _, err := os.Stat(filepath.Join(outDir, "types.d.ts")); err == nil {
		t.Fatalf("did not expect types.d.ts to be copied")
	}
}

func TestEnsureWithinOutDir(t *testing.T) {
	root := t.TempDir()
	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := ensureWithinOutDir(outDir, filepath.Join(outDir, "ok.js")); err != nil {
		t.Fatalf("expected ok path, got error: %v", err)
	}
	if err := ensureWithinOutDir(outDir, filepath.Join(outDir, "..", "evil.js")); err == nil {
		t.Fatalf("expected escape error, got nil")
	}
}

func TestWarnImportExport(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	source := "import foo from './foo'\nconst a = 1\nexport function bar() {}\n// export ignored\n"
	warnImportExport("src/Main.ts", source)

	output := buf.String()
	if !strings.Contains(output, "src/Main.ts:1") {
		t.Fatalf("expected warning for line 1, got: %s", output)
	}
	if !strings.Contains(output, "src/Main.ts:3") {
		t.Fatalf("expected warning for line 3, got: %s", output)
	}
}

func TestTargetFilter(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "nested"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	file := filepath.Join(root, "src", "nested", "Code.gs")
	if err := os.WriteFile(file, []byte("function a() {}"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	filter, err := NewTargetFilter(root, []string{"src/nested"})
	if err != nil {
		t.Fatalf("NewTargetFilter failed: %v", err)
	}
	if filter == nil || filter.Empty() {
		t.Fatalf("expected non-empty filter")
	}
	if !filterAllows(filter, "src/nested/Code.gs") {
		t.Fatalf("expected path to be allowed")
	}
	if filterAllows(filter, "src/other/Code.gs") {
		t.Fatalf("expected unrelated path to be rejected")
	}
}
