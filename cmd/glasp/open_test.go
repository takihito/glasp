package main

import (
	"testing"

	"github.com/takihito/glasp/internal/config"
)

func TestOpenScriptCommandFlow(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}

	var openedURL string
	origOpen := openURLFn
	t.Cleanup(func() { openURLFn = origOpen })
	openURLFn = func(url string) error {
		openedURL = url
		return nil
	}

	if err := (&OpenScriptCmd{}).Run(nil); err != nil {
		t.Fatalf("OpenScriptCmd.Run failed: %v", err)
	}
	if openedURL != "https://script.google.com/d/script-id/edit" {
		t.Fatalf("unexpected opened URL: %s", openedURL)
	}
}
