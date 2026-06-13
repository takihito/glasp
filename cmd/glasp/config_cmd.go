package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/takihito/glasp/internal/config"
)

// ConfigCmd represents the 'config' subcommand.
type ConfigCmd struct {
	Init ConfigInitCmd `cmd:"" help:"Create .glasp/config.json."`
}

// ConfigInitCmd represents the 'config init' subcommand.
type ConfigInitCmd struct{}

// Run executes the config init command.
func (c *ConfigInitCmd) Run(rc *runContext) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	configPath := filepath.Join(projectRoot, ".glasp", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config already exists: %s", configPath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check %s: %w", configPath, err)
	}
	cfg := &config.GlaspConfig{
		Archive: config.ArchiveConfig{
			Pull: false,
			Push: false,
		},
	}
	if err := config.SaveGlaspConfig(projectRoot, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created config at %s\n", configPath)
	return nil
}
