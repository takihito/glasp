package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"

	"github.com/alecthomas/kong"
)

// CreateCmd represents the 'create' subcommand.
type CreateCmd struct {
	Title         string `help:"Title of the Apps Script project."`
	Type          string `help:"Type of the Apps Script project (e.g., standalone, sheet, doc)." default:"standalone"`
	RootDir       string `name:"rootDir" help:"Root directory for project files." default:"./"`
	ParentID      string `name:"parentId" help:"Parent Drive file ID for container-bound scripts."`
	FileExtension string `help:"Script file extension (e.g., js, gs, ts)." default:"js"`
	Auth          string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the create command.
func (c *CreateCmd) Run(ctx *kong.Context) error {
	title, err := validateTitle(c.Title)
	if err != nil {
		return err
	}
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	projectType := strings.ToLower(strings.TrimSpace(c.Type))
	if projectType == "" {
		projectType = "standalone"
	}
	if err := validateCreateType(projectType); err != nil {
		return err
	}
	parentID := strings.TrimSpace(c.ParentID)
	if parentID == "" && projectType != "standalone" {
		return fmt.Errorf("project type %q is not supported yet; currently only \"standalone\" is supported without --parentId", projectType)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	if err := ensureNoExistingClaspConfig(projectRoot); err != nil {
		return err
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}

	project, err := client.CreateProject(context.Background(), title, parentID)
	if err != nil {
		return err
	}
	cfg := &config.ClaspConfig{
		ScriptID: project.ScriptId,
		ParentID: project.ParentId,
	}
	cfg.RootDir = strings.TrimSpace(c.RootDir)
	if cfg.RootDir == "" {
		cfg.RootDir = "./"
	}
	fileExt := strings.TrimSpace(c.FileExtension)
	if fileExt != "" {
		fileExt = strings.TrimPrefix(strings.ToLower(fileExt), ".")
		if err := validateSupportedSyncFileExtension(fileExt); err != nil {
			return err
		}
		cfg.Extra = map[string]json.RawMessage{
			"fileExtension": json.RawMessage(fmt.Sprintf("%q", fileExt)),
		}
	}
	if err := config.SaveClaspConfig(projectRoot, cfg); err != nil {
		return err
	}

	content, err := client.GetContent(context.Background(), project.ScriptId, 0)
	if err != nil {
		return err
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, nil)
	if err != nil {
		return err
	}
	if _, err := syncer.ApplyRemoteContent(opts, content); err != nil {
		return err
	}

	fmt.Printf("Created project %s\n", project.ScriptId)
	return nil
}
