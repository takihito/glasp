package main

import (
	"context"
	"fmt"

	"github.com/takihito/glasp/internal/archive"
	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"

	"github.com/alecthomas/kong"
)

// PullCmd represents the 'pull' subcommand.
type PullCmd struct {
	Archive bool   `help:"Save pulled files under .glasp/archive/<scriptId>/pull/YYYYMMDD_HHMMSS."`
	DryRun  bool   `name:"dryrun" help:"Run conversion planning only without API calls or local writes."`
	Auth    string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the pull command.
func (c *PullCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	pc, err := loadProjectContext()
	if err != nil {
		return err
	}
	projectRoot, cfg, scriptID := pc.Root, pc.Config, pc.ScriptID
	fileExtension := claspFileExtension(cfg)
	if err := validateSupportedSyncFileExtension(fileExtension); err != nil {
		return err
	}
	glaspCfg, err := config.LoadGlaspConfig(projectRoot)
	if err != nil {
		return err
	}
	archiveEnabled := c.Archive
	if glaspCfg != nil && glaspCfg.Archive.Pull {
		archiveEnabled = true
	}
	setRunArchiveMeta(archiveEnabled, "pull")
	if c.DryRun {
		fmt.Printf("Dry run pull for project %s (convert=%s): skipped API fetch and local file writes\n", scriptID, dryRunConvertLabelForPull(fileExtension))
		return nil
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	content, err := client.GetContent(context.Background(), scriptID, 0)
	if err != nil {
		return err
	}
	pullMode := transform.Mode("")
	workingContent := content
	if isTypeScriptFileExtension(fileExtension) {
		pullMode = transform.ModeGasToTS
		workingContent, err = convertPulledContentFn(content, projectRoot)
		if err != nil {
			return err
		}
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, nil)
	if err != nil {
		return err
	}
	if archiveEnabled {
		archiveRoot, err := archive.PullRun(projectRoot, scriptID, cfg, content, workingContent, fileExtension, pullMode)
		if err != nil {
			return err
		}
		setRunArchivePath(archiveRoot)
		fmt.Printf("Archived pull to %s\n", archiveRoot)
	}
	if _, err := syncer.ApplyRemoteContent(opts, workingContent); err != nil {
		return err
	}

	fmt.Printf("Pulled project %s\n", scriptID)
	return nil
}
