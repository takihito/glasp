package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"
)

func TestArchivePushRunWritesPayloadIndex(t *testing.T) {
	root := t.TempDir()
	workingFiles := []syncer.ProjectFile{
		{LocalPath: "src/Code.ts", RemotePath: "Code", Type: "SERVER_JS", Source: "const x = 1"},
	}
	payloadFiles := []syncer.ProjectFile{
		{LocalPath: "src/Code.js", RemotePath: "Code", Type: "SERVER_JS", Source: "function x() {}"},
		{LocalPath: "appsscript.json", RemotePath: "appsscript", Type: "JSON", Source: "{}"},
	}
	archiveRoot, err := archivePushRun(root, "script-id", workingFiles, payloadFiles, "ts", transform.ModeTSToGas)
	if err != nil {
		t.Fatalf("archivePushRun failed: %v", err)
	}
	manifestPath := filepath.Join(archiveRoot, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest failed: %v", err)
	}
	var manifest archiveManifest
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
