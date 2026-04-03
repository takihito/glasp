package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadSaveClaspConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glasp_config_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalConfig := &ClaspConfig{
		ScriptID:  "testScriptId123",
		RootDir:   "src",
		ProjectID: "project-123",
		ParentID:  "parent-123",
	}
	err = SaveClaspConfig(tmpDir, originalConfig)
	if err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	loadedConfig, err := LoadClaspConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadClaspConfig failed: %v", err)
	}

	if loadedConfig.ScriptID != originalConfig.ScriptID {
		t.Errorf("ScriptID mismatch: expected %s, got %s", originalConfig.ScriptID, loadedConfig.ScriptID)
	}
	if loadedConfig.RootDir != originalConfig.RootDir {
		t.Errorf("RootDir mismatch: expected %s, got %s", originalConfig.RootDir, loadedConfig.RootDir)
	}
	if loadedConfig.ProjectID != originalConfig.ProjectID {
		t.Errorf("ProjectID mismatch: expected %s, got %s", originalConfig.ProjectID, loadedConfig.ProjectID)
	}
	if loadedConfig.ParentID != originalConfig.ParentID {
		t.Errorf("ParentID mismatch: expected %s, got %s", originalConfig.ParentID, loadedConfig.ParentID)
	}
}

func TestLoadClaspConfigNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glasp_config_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = LoadClaspConfig(tmpDir)
	if err == nil {
		t.Error("LoadClaspConfig did not return an error for a non-existent file")
	}
	if !strings.Contains(err.Error(), ".clasp.json not found") {
		t.Errorf("Expected 'not found' error, got %v", err)
	}
}

func TestClaspIgnore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glasp_ignore_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with no .claspignore file
	ci, err := NewClaspIgnore(tmpDir)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed when no file exists: %v", err)
	}
	if ci.Matches("any/file.js") {
		t.Error("Expected no match for no .claspignore file, but got a match")
	}
	if !ci.Matches(".glasp/archive/file.js") {
		t.Error("Expected .glasp/ to be ignored by default")
	}
	if !ci.Matches("node_modules/lib/file.js") {
		t.Error("Expected node_modules/ to be ignored by default")
	}

	// Test with a .claspignore file
	ignoreContent := "# Comments are ignored\r\n**/*.js\r\n!src/main.js\r\n\r\n/dist\r\nnode_modules/\r\n"
	err = os.WriteFile(filepath.Join(tmpDir, ".claspignore"), []byte(ignoreContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write .claspignore: %v", err)
	}

	ci, err = NewClaspIgnore(tmpDir)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed for a valid file: %v", err)
	}

	testCases := []struct {
		path    string
		ignored bool
		name    string
	}{
		{"file.js", true, "js file in root"},
		{"src/file.js", true, "nested js file"},
		{"src/main.js", false, "negated js file"},
		{"dist/bundle.js", true, "file in root-anchored ignored dir"},
		{"dist/sub/bundle.js", true, "nested file in root-anchored ignored dir"},
		{"src/dist/bundle.js", true, "dist directory not at root is still ignored by **/*.js"},
		{"node_modules/lib/file.js", true, "file in ignored node_modules"},
		{"src/node_modules/lib/file.js", true, "nested node_modules is also ignored by **/*.js"},
		{".glasp/archive/file.js", true, "default glasp archive directory is ignored"},
		{"appsscript.json", false, "unrelated file should not be ignored"},
		{"README.md", false, "another unrelated file should not be ignored"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ci.Matches(tc.path); got != tc.ignored {
				t.Errorf("Path '%s': expected ignored=%t, but got %t", tc.path, tc.ignored, got)
			}
		})
	}
}

func TestClaspConfigPreservesExtraFields(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glasp_config_extra_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rawConfig := `{
  "scriptId": "testScriptId123",
  "rootDir": "src",
  "projectId": "project-123",
  "parentId": "parent-123",
  "fileExtension": "ts"
}`
	configPath := filepath.Join(tmpDir, ".clasp.json")
	if err := os.WriteFile(configPath, []byte(rawConfig), 0644); err != nil {
		t.Fatalf("Failed to write .clasp.json: %v", err)
	}

	cfg, err := LoadClaspConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadClaspConfig failed: %v", err)
	}
	if cfg.Extra == nil {
		t.Fatalf("Expected Extra fields, got nil")
	}
	extraValue, ok := cfg.Extra["fileExtension"]
	if !ok {
		t.Fatalf("Expected fileExtension to be preserved in Extra")
	}
	if string(extraValue) != `"ts"` {
		t.Fatalf("Expected fileExtension to be \"ts\", got %s", string(extraValue))
	}

	if err := SaveClaspConfig(tmpDir, cfg); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	savedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved .clasp.json: %v", err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(savedData, &saved); err != nil {
		t.Fatalf("Failed to unmarshal saved .clasp.json: %v", err)
	}
	if _, ok := saved["fileExtension"]; !ok {
		t.Fatalf("Expected fileExtension to be preserved after save")
	}
	if string(saved["fileExtension"]) != `"ts"` {
		t.Fatalf("Expected fileExtension to be \"ts\" after save, got %s", string(saved["fileExtension"]))
	}
}

func TestLoadClaspConfigFromDocsSample(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to determine current test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	docsDir := filepath.Join(repoRoot, "docs")

	cfg, err := LoadClaspConfig(docsDir)
	if err != nil {
		t.Fatalf("LoadClaspConfig failed: %v", err)
	}
	if cfg.ScriptID != "1xxxxxxxxxxxxxxxxxxxxx_yyyyyyyyyyyyyyyyyyyyy" {
		t.Fatalf("ScriptID mismatch: got %s", cfg.ScriptID)
	}
	if cfg.RootDir != "src/" {
		t.Fatalf("RootDir mismatch: got %s", cfg.RootDir)
	}
	if cfg.ProjectID != "project-id-xxxxxxxxxxxxxxxxxxx" {
		t.Fatalf("ProjectID mismatch: got %s", cfg.ProjectID)
	}
	if cfg.Extra == nil {
		t.Fatalf("Expected Extra fields, got nil")
	}
	if value, ok := cfg.Extra["fileExtension"]; !ok || string(value) != `"ts"` {
		t.Fatalf("fileExtension mismatch: got %s", string(value))
	}
	rawOrder, ok := cfg.Extra["filePushOrder"]
	if !ok {
		t.Fatalf("Expected filePushOrder to be preserved")
	}
	var order []string
	if err := json.Unmarshal(rawOrder, &order); err != nil {
		t.Fatalf("Failed to unmarshal filePushOrder: %v", err)
	}
	if len(order) != 2 || order[0] != "file1.ts" || order[1] != "file2.ts" {
		t.Fatalf("filePushOrder mismatch: got %v", order)
	}
}

func TestClaspIgnoreFromDocsSample(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to determine current test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	docsDir := filepath.Join(repoRoot, "docs")

	ci, err := NewClaspIgnore(docsDir)
	if err != nil {
		t.Fatalf("NewClaspIgnore failed: %v", err)
	}

	testCases := []struct {
		path    string
		ignored bool
		name    string
	}{
		{"appsscript.json", false, "appsscript.json is explicitly allowed"},
		{"src/main.gs", false, "gs files are allowed"},
		{"src/main.js", true, "non-gs files are ignored"},
		{"README.md", true, "non-gs files at root are ignored"},
		{"node_modules/pkg/index.gs", true, "node_modules is ignored even for allowed extensions"},
		{".git/config", true, ".git is ignored"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ci.Matches(tc.path); got != tc.ignored {
				t.Errorf("Path '%s': expected ignored=%t, but got %t", tc.path, tc.ignored, got)
			}
		})
	}
}
