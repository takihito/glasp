package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takihito/glasp/internal/config"
)

func TestConfigInitCreatesConfig(t *testing.T) {
	root := useTempDir(t)
	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("ConfigInitCmd.Run failed: %v", err)
	}

	cfgPath := filepath.Join(root, ".glasp", "config.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config.json to exist: %v", err)
	}
	cfg, err := config.LoadGlaspConfig(root)
	if err != nil {
		t.Fatalf("LoadGlaspConfig failed: %v", err)
	}
	if cfg.Archive.Pull {
		t.Fatalf("expected archive.pull to be false by default")
	}
	if cfg.Archive.Push {
		t.Fatalf("expected archive.push to be false by default")
	}
}

func TestConfigInitRejectsExistingConfig(t *testing.T) {
	root := useTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, ".glasp"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".glasp", "config.json"), []byte(`{"archive":{"pull":true}}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err == nil {
		t.Fatalf("expected error when config already exists")
	}
}

func TestConfigInitCreatesConfigWithoutClaspConfig(t *testing.T) {
	root := useTempDir(t)
	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("expected config init to work without .clasp.json in %s: %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".glasp", "config.json")); err != nil {
		t.Fatalf("expected config.json to exist: %v", err)
	}
}

// readClaspJSON returns the raw key/value pairs of .clasp.json in dir.
func readClaspJSON(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".clasp.json"))
	if err != nil {
		t.Fatalf("read .clasp.json failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal .clasp.json failed: %v", err)
	}
	return raw
}

func TestConfigInitCreatesEmptyClaspConfig(t *testing.T) {
	root := useTempDir(t)
	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("ConfigInitCmd.Run failed: %v", err)
	}

	raw := readClaspJSON(t, root)
	want := map[string]any{"scriptId": ""}
	if !reflect.DeepEqual(raw, want) {
		t.Fatalf("unexpected .clasp.json: got %#v, want %#v", raw, want)
	}
}

func TestConfigInitWritesClaspOptions(t *testing.T) {
	root := useTempDir(t)
	cmd := ConfigInitCmd{RootDir: "src", FileExtension: ".TS", ScriptID: "abc_DEF-123"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("ConfigInitCmd.Run failed: %v", err)
	}

	raw := readClaspJSON(t, root)
	want := map[string]any{"scriptId": "abc_DEF-123", "rootDir": "src", "fileExtension": "ts"}
	if !reflect.DeepEqual(raw, want) {
		t.Fatalf("unexpected .clasp.json: got %#v, want %#v", raw, want)
	}
}

func TestConfigInitCreatesClaspConfigWhenOnlyGlaspConfigExists(t *testing.T) {
	root := useTempDir(t)
	cfgPath := filepath.Join(root, ".glasp", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	original := []byte(`{"archive":{"pull":true}}`)
	if err := os.WriteFile(cfgPath, original, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cmd := ConfigInitCmd{ScriptID: "script-id"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("ConfigInitCmd.Run failed: %v", err)
	}

	if got := readClaspJSON(t, root)["scriptId"]; got != "script-id" {
		t.Fatalf("expected scriptId script-id, got %#v", got)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config.json failed: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("expected existing config.json to be left untouched, got %s", data)
	}
}

func TestConfigInitKeepsExistingClaspConfig(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "existing", RootDir: "src"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	cmd := ConfigInitCmd{}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("ConfigInitCmd.Run failed: %v", err)
	}

	raw := readClaspJSON(t, root)
	want := map[string]any{"scriptId": "existing", "rootDir": "src"}
	if !reflect.DeepEqual(raw, want) {
		t.Fatalf("expected existing .clasp.json to be left untouched: got %#v", raw)
	}
	if _, err := os.Stat(filepath.Join(root, ".glasp", "config.json")); err != nil {
		t.Fatalf("expected config.json to be created: %v", err)
	}
}

func TestConfigInitRejectsOptionsWhenClaspConfigExists(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "existing"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	cmd := ConfigInitCmd{ScriptID: "other"}
	if err := cmd.Run(nil); err == nil {
		t.Fatalf("expected error when options are given for an existing .clasp.json")
	}
	if got := readClaspJSON(t, root)["scriptId"]; got != "existing" {
		t.Fatalf("expected .clasp.json to be left untouched, got scriptId %#v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".glasp", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no config.json on validation failure, stat err=%v", err)
	}
}

func TestConfigInitRejectsInvalidOptions(t *testing.T) {
	cases := map[string]ConfigInitCmd{
		"invalid script id":  {ScriptID: "abc/def"},
		"absolute rootDir":   {RootDir: "/tmp/src"},
		"rootDir outside":    {RootDir: "../src"},
		"rootDir parent":     {RootDir: "src/../.."},
		"tsx file extension": {FileExtension: "tsx"},
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			root := useTempDir(t)
			if err := cmd.Run(nil); err == nil {
				t.Fatalf("expected error for %+v", cmd)
			}
			if _, err := os.Stat(filepath.Join(root, ".clasp.json")); !os.IsNotExist(err) {
				t.Fatalf("expected no .clasp.json on validation failure, stat err=%v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".glasp")); !os.IsNotExist(err) {
				t.Fatalf("expected no .glasp directory on validation failure, stat err=%v", err)
			}
		})
	}
}

func TestConfigInitInsideParentProjectThenLoginUsesCurrentDir(t *testing.T) {
	parent := useTempDir(t)
	if err := config.SaveClaspConfig(parent, &config.ClaspConfig{}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	parentToken := filepath.Join(parent, ".glasp", "access.json")
	if err := os.MkdirAll(filepath.Dir(parentToken), 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(parentToken, []byte(`{"token":{"access_token":"parent"}}`), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	proj := filepath.Join(parent, "proj")
	if err := os.MkdirAll(proj, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.Chdir(proj); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	var errBuf bytes.Buffer
	origStderr := stderr
	stderr = &errBuf
	t.Cleanup(func() { stderr = origStderr })

	initCmd := ConfigInitCmd{}
	if err := initCmd.Run(nil); err != nil {
		t.Fatalf("ConfigInitCmd.Run failed: %v", err)
	}
	if !strings.Contains(errBuf.String(), parent) {
		t.Fatalf("expected note about parent project %s, got %q", parent, errBuf.String())
	}

	loginCmd := LoginCmd{Auth: writeAuthFixture(t, parent)}
	if err := loginCmd.Run(nil); err != nil {
		t.Fatalf("LoginCmd.Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj, ".glasp", "access.json")); err != nil {
		t.Fatalf("expected token cache under current directory: %v", err)
	}
}
