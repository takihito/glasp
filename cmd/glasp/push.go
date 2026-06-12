package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/takihito/glasp/internal/archive"
	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/history"
	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"

	"github.com/alecthomas/kong"
)

// PushCmd represents the 'push' subcommand.
type PushCmd struct {
	Watch     bool   `help:"Watch for local file changes and push automatically. TODO implement"`
	Force     bool   `help:"Force push all files, ignoring .claspignore and default excludes (use with caution). By default node_modules/ is excluded even without .claspignore."`
	Archive   bool   `help:"Save pushed files under .glasp/archive/<scriptId>/push/YYYYMMDD_HHMMSS."`
	DryRun    bool   `name:"dryrun" help:"Run conversion only without API calls or remote update."`
	HistoryID int64  `name:"history-id" help:"Push using payload archived by the specified history ID."`
	Auth      string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the push command.
func (c *PushCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	if c.Watch {
		return fmt.Errorf("watch mode is not implemented yet")
	}
	projectRoot, err := findExistingProjectRoot()
	if err != nil {
		return err
	}
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ScriptID) == "" {
		return fmt.Errorf("script ID is required in .clasp.json")
	}
	fileExtension := claspFileExtension(cfg)
	if err := validateSupportedSyncFileExtension(fileExtension); err != nil {
		return err
	}
	scriptID, err := validateScriptID(cfg.ScriptID)
	if err != nil {
		return err
	}
	glaspCfg, err := config.LoadGlaspConfig(projectRoot)
	if err != nil {
		return err
	}
	archiveEnabled := c.Archive
	if glaspCfg != nil && glaspCfg.Archive.Push {
		archiveEnabled = true
	}
	setRunArchiveMeta(archiveEnabled, "push")
	if c.HistoryID < 0 {
		return fmt.Errorf("history-id must be >= 0")
	}
	if c.HistoryID > 0 {
		if c.Force {
			return fmt.Errorf("--history-id cannot be combined with --force")
		}
		return c.runFromHistoryID(projectRoot, scriptID, authPath, archiveEnabled, cfg.RootDir)
	}

	var ignore *config.ClaspIgnore
	if !c.Force {
		ignore, err = config.NewClaspIgnore(projectRoot)
		if err != nil {
			return err
		}
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, ignore)
	if err != nil {
		return err
	}
	files, err := syncer.CollectLocalFiles(opts)
	if err != nil {
		return err
	}
	syncer.SortFilesByPushOrder(files, opts.FilePushOrder, opts.RootDir)
	// Enable TS→GAS transpilation when fileExtension is "ts" or when
	// collected files contain TypeScript sources.
	pushMode := transform.Mode("")
	if isTypeScriptFileExtension(fileExtension) || hasTypeScriptFiles(files) {
		pushMode = transform.ModeTSToGas
	}
	payloadFiles := files
	if pushMode == transform.ModeTSToGas {
		payloadFiles, err = convertPushFiles(files, projectRoot)
		if err != nil {
			return err
		}
	}
	content := syncer.BuildContent(payloadFiles)
	if c.DryRun {
		fmt.Printf("Dry run push for project %s (convert=%s): prepared %d files, skipped API update and archive writes\n", scriptID, pushMode.Label(), len(content.Files))
		return nil
	}

	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	if _, err := client.UpdateContent(context.Background(), scriptID, content); err != nil {
		return err
	}
	if archiveEnabled {
		archiveRoot, err := archive.PushRun(projectRoot, scriptID, files, payloadFiles, fileExtension, pushMode)
		if err != nil {
			return err
		}
		setRunArchivePath(archiveRoot)
		fmt.Printf("Archived push to %s\n", archiveRoot)
	}

	fmt.Printf("Pushed project %s\n", scriptID)
	return nil
}

func (c *PushCmd) runFromHistoryID(projectRoot, scriptID, authPath string, archiveEnabled bool, rootDir string) error {
	entry, found, err := history.GetByID(projectRoot, c.HistoryID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("history id %d not found", c.HistoryID)
	}
	if !entry.Archive.Enabled {
		return fmt.Errorf("history id %d has archive disabled", c.HistoryID)
	}
	if cmd := strings.TrimSpace(entry.Command); cmd != "" && cmd != "push" {
		return fmt.Errorf("history id %d command is %q (expected push)", c.HistoryID, cmd)
	}
	if strings.TrimSpace(entry.Archive.Direction) != "push" {
		return fmt.Errorf("history id %d is not a push archive entry", c.HistoryID)
	}
	archivePath := strings.TrimSpace(entry.Archive.Path)
	if archivePath == "" {
		return fmt.Errorf("history id %d has empty archive path", c.HistoryID)
	}

	manifest, payloadFiles, err := archive.LoadPushPayload(archivePath, rootDir)
	if err != nil {
		return fmt.Errorf("failed to load archive for history id %d: %w", c.HistoryID, err)
	}
	if strings.TrimSpace(manifest.ScriptID) == "" {
		return fmt.Errorf("archive manifest scriptId is empty: %s", filepath.Join(archivePath, "manifest.json"))
	}
	if manifest.ScriptID != scriptID {
		return fmt.Errorf("archive scriptId mismatch: history has %s but current project is %s", manifest.ScriptID, scriptID)
	}
	fmt.Printf(
		"Using history source id=%d archive.direction=%s archive.path=%s manifest.timestamp=%s\n",
		c.HistoryID,
		entry.Archive.Direction,
		archivePath,
		manifest.Timestamp,
	)
	content := syncer.BuildContent(payloadFiles)
	if c.DryRun {
		fmt.Printf("Dry run push from history id %d for project %s: prepared %d files from %s, skipped API update\n", c.HistoryID, scriptID, len(content.Files), archivePath)
		return nil
	}

	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	if _, err := client.UpdateContent(context.Background(), scriptID, content); err != nil {
		return err
	}
	if archiveEnabled {
		archiveRoot, err := archive.PushRun(projectRoot, scriptID, payloadFiles, payloadFiles, manifest.FileExtension, transform.ModeFromLabel(manifest.Convert))
		if err != nil {
			return err
		}
		setRunArchivePath(archiveRoot)
		fmt.Printf("Archived push to %s\n", archiveRoot)
	}
	fmt.Printf("Pushed project %s from history id %d\n", scriptID, c.HistoryID)
	return nil
}
