package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"
)

// CloneCmd represents the 'clone' subcommand.
type CloneCmd struct {
	ScriptID      string `arg:"" help:"Script ID of the Apps Script project to clone."`
	RootDir       string `name:"rootDir" help:"Root directory for cloned project files." default:"./"`
	FileExtension string `help:"Script file extension for cloned files (e.g., js, gs, ts)." default:"js"`
	Auth          string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the clone command.
func (c *CloneCmd) Run(rc *runContext) error {
	scriptID, err := validateScriptID(c.ScriptID)
	if err != nil {
		return err
	}
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	fileExt := normalizeFileExtension(c.FileExtension)
	if fileExt == "" {
		fileExt = "js"
	}
	if err := validateSupportedSyncFileExtension(fileExt); err != nil {
		return err
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	if err := ensureNoExistingClaspConfig(projectRoot); err != nil {
		return err
	}
	client, err := newProjectScriptClient(rc.Context(), projectRoot, authPath)
	if err != nil {
		return err
	}
	project, err := client.GetProject(rc.Context(), scriptID)
	if err != nil {
		return err
	}
	cfg := &config.ClaspConfig{
		ScriptID: scriptID,
		ParentID: project.ParentId,
	}
	cfg.RootDir = strings.TrimSpace(c.RootDir)
	if cfg.RootDir == "" {
		cfg.RootDir = "./"
	}
	cfg.Extra = map[string]json.RawMessage{
		"fileExtension": json.RawMessage(fmt.Sprintf("%q", fileExt)),
	}
	if err := config.SaveClaspConfig(projectRoot, cfg); err != nil {
		return err
	}
	content, err := client.GetContent(rc.Context(), scriptID, 0)
	if err != nil {
		return err
	}
	fileExtension := claspFileExtension(cfg)
	workingContent := content
	if isTypeScriptFileExtension(fileExtension) {
		workingContent, err = convertPulledContentFn(content, projectRoot)
		if err != nil {
			return err
		}
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, nil)
	if err != nil {
		return err
	}
	if _, err := syncer.ApplyRemoteContent(opts, workingContent); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Cloned project %s\n", scriptID)
	return nil
}
