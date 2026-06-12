package archive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"
)

func TestPushRunWritesPayloadIndex(t *testing.T) {
	root := t.TempDir()
	workingFiles := []syncer.ProjectFile{
		{LocalPath: "src/Code.ts", RemotePath: "Code", Type: "SERVER_JS", Source: "const x = 1"},
	}
	payloadFiles := []syncer.ProjectFile{
		{LocalPath: "src/Code.js", RemotePath: "Code", Type: "SERVER_JS", Source: "function x() {}"},
		{LocalPath: "appsscript.json", RemotePath: "appsscript", Type: "JSON", Source: "{}"},
	}
	archiveRoot, err := PushRun(root, "script-id", workingFiles, payloadFiles, "ts", transform.ModeTSToGas)
	if err != nil {
		t.Fatalf("PushRun failed: %v", err)
	}
	manifestPath := filepath.Join(archiveRoot, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest failed: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest failed: %v", err)
	}
	if len(manifest.PayloadIndex) != 2 {
		t.Fatalf("expected 2 payloadIndex entries, got %d", len(manifest.PayloadIndex))
	}
	if manifest.PayloadIndex[0].Path != "appsscript.json" || manifest.PayloadIndex[0].RemotePath != "appsscript" {
		t.Fatalf("unexpected first payloadIndex entry: %#v", manifest.PayloadIndex[0])
	}
	if manifest.PayloadIndex[1].Path != "src/Code.js" || manifest.PayloadIndex[1].RemotePath != "Code" {
		t.Fatalf("unexpected second payloadIndex entry: %#v", manifest.PayloadIndex[1])
	}
}

func TestPushRunThenLoadPushPayloadRoundTrip(t *testing.T) {
	root := t.TempDir()
	payloadFiles := []syncer.ProjectFile{
		{LocalPath: "src/Code.js", RemotePath: "Code", Type: "SERVER_JS", Source: "function x() {}"},
		{LocalPath: "appsscript.json", RemotePath: "appsscript", Type: "JSON", Source: "{}"},
	}
	archiveRoot, err := PushRun(root, "script-id", payloadFiles, payloadFiles, "js", transform.Mode(""))
	if err != nil {
		t.Fatalf("PushRun failed: %v", err)
	}

	manifest, loaded, err := LoadPushPayload(archiveRoot, "src")
	if err != nil {
		t.Fatalf("LoadPushPayload failed: %v", err)
	}
	if manifest.ScriptID != "script-id" || manifest.Direction != "push" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if manifest.Convert != "none" {
		t.Fatalf("expected convert label none, got %q", manifest.Convert)
	}
	if manifest.Status != "success" {
		t.Fatalf("expected status success, got %q", manifest.Status)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 payload files, got %d", len(loaded))
	}
	if loaded[0].LocalPath != "appsscript.json" || loaded[0].RemotePath != "appsscript" || loaded[0].Type != "JSON" {
		t.Fatalf("unexpected first payload file: %#v", loaded[0])
	}
	if loaded[1].LocalPath != "src/Code.js" || loaded[1].RemotePath != "Code" || loaded[1].Type != "SERVER_JS" {
		t.Fatalf("unexpected second payload file: %#v", loaded[1])
	}
	if loaded[1].Source != "function x() {}" {
		t.Fatalf("unexpected payload source: %q", loaded[1].Source)
	}
}

func TestLoadPushPayloadWithoutIndexStripsRootDirPrefix(t *testing.T) {
	root := t.TempDir()
	payloadDir := filepath.Join(root, "payload", "src")
	if err := os.MkdirAll(payloadDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(payloadDir, "Code.js"), []byte("function x() {}"), 0644); err != nil {
		t.Fatalf("write payload failed: %v", err)
	}
	manifest := Manifest{ScriptID: "script-id", Direction: "push", Status: "success"}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), data, 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	_, loaded, err := LoadPushPayload(root, "src")
	if err != nil {
		t.Fatalf("LoadPushPayload failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 payload file, got %d", len(loaded))
	}
	if loaded[0].RemotePath != "Code" {
		t.Fatalf("expected rootDir prefix stripped from remote path, got %q", loaded[0].RemotePath)
	}
}

func TestLoadPushPayloadRejectsNonPushDirection(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{ScriptID: "script-id", Direction: "pull", Status: "success"}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), data, 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if _, _, err := LoadPushPayload(root, ""); err == nil {
		t.Fatalf("expected error for non-push direction")
	}
}
