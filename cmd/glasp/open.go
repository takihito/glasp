package main

import (
	"fmt"
	"strings"

	"github.com/takihito/glasp/internal/config"

	"github.com/alecthomas/kong"
)

// OpenScriptCmd represents the 'open-script' subcommand.
type OpenScriptCmd struct {
	ScriptID string `name:"scriptId" help:"Script ID override. If omitted, uses .clasp.json."`
}

// Run executes the open-script command.
func (c *OpenScriptCmd) Run(ctx *kong.Context) error {
	scriptID := strings.TrimSpace(c.ScriptID)
	if scriptID == "" {
		projectRoot, err := findExistingProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.LoadClaspConfig(projectRoot)
		if err != nil {
			return err
		}
		scriptID = cfg.ScriptID
	}
	validatedScriptID, err := validateScriptID(scriptID)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://script.google.com/d/%s/edit", validatedScriptID)
	if err := openURLFn(url); err != nil {
		fmt.Println(url)
		return fmt.Errorf("failed to open browser: %w", err)
	}
	fmt.Printf("Opened %s\n", url)
	return nil
}
