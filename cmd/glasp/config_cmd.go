package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/takihito/glasp/internal/config"
)

// ConfigCmd represents the 'config' subcommand.
type ConfigCmd struct {
	Init ConfigInitCmd `cmd:"" help:"Create .glasp/config.json and .clasp.json in the current directory."`
}

// ConfigInitCmd represents the 'config init' subcommand.
type ConfigInitCmd struct {
	RootDir       string `name:"rootDir" help:"rootDir written to the new .clasp.json (relative path inside the project)."`
	FileExtension string `help:"fileExtension written to the new .clasp.json (e.g., js, gs, ts)."`
	ScriptID      string `name:"script-id" help:"scriptId written to the new .clasp.json."`
}

// hasClaspOptions reports whether any .clasp.json field was given.
func (c *ConfigInitCmd) hasClaspOptions() bool {
	return c.RootDir != "" || c.FileExtension != "" || c.ScriptID != ""
}

// Run executes the config init command.
// The current directory becomes the project root: each of .glasp/config.json
// and .clasp.json is created only when missing, so running it in a directory
// that already has one of them fills in the other. Creating .clasp.json here
// keeps later commands (e.g. login) from resolving the project root to a
// .clasp.json in a parent directory.
func (c *ConfigInitCmd) Run(rc *runContext) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	configPath := filepath.Join(projectRoot, ".glasp", "config.json")
	claspPath := filepath.Join(projectRoot, ".clasp.json")

	configExists, err := fileExists(configPath)
	if err != nil {
		return err
	}
	claspExists, err := fileExists(claspPath)
	if err != nil {
		return err
	}

	// Validate everything before writing so a bad option leaves no partial state.
	if claspExists && c.hasClaspOptions() {
		return fmt.Errorf(".clasp.json already exists: %s; --rootDir/--file-extension/--script-id are only applied when creating it", claspPath)
	}
	if configExists && claspExists {
		return fmt.Errorf("config already exists: %s and %s", configPath, claspPath)
	}
	var claspCfg *config.ClaspConfig
	if !claspExists {
		claspCfg, err = c.buildClaspConfig()
		if err != nil {
			return err
		}
	}

	if !configExists {
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
	}

	if claspCfg != nil {
		parentRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}
		if err := config.SaveClaspConfig(projectRoot, claspCfg); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created .clasp.json at %s\n", claspPath)
		if parentRoot != "" && filepath.Clean(parentRoot) != filepath.Clean(projectRoot) {
			fmt.Fprintf(stderr, "Note: a parent project also exists at %s; commands run here now use %s\n", parentRoot, projectRoot)
		}
	}
	return nil
}

// buildClaspConfig validates the options and returns the .clasp.json to write.
func (c *ConfigInitCmd) buildClaspConfig() (*config.ClaspConfig, error) {
	cfg := &config.ClaspConfig{}
	if c.ScriptID != "" {
		scriptID, err := validateScriptID(c.ScriptID)
		if err != nil {
			return nil, err
		}
		cfg.ScriptID = scriptID
	}
	if c.RootDir != "" {
		rootDir, err := validateConfigRootDir(c.RootDir)
		if err != nil {
			return nil, err
		}
		cfg.RootDir = rootDir
	}
	if c.FileExtension != "" {
		fileExt := normalizeFileExtension(c.FileExtension)
		if fileExt == "" {
			return nil, fmt.Errorf("invalid fileExtension %q", c.FileExtension)
		}
		encoded, err := json.Marshal(fileExt)
		if err != nil {
			return nil, fmt.Errorf("failed to encode fileExtension: %w", err)
		}
		if err := validateSupportedSyncFileExtension(fileExt); err != nil {
			return nil, err
		}
		cfg.Extra = map[string]json.RawMessage{
			"fileExtension": json.RawMessage(encoded),
		}
	}
	return cfg, nil
}

// validateConfigRootDir accepts only relative paths that stay inside the
// project root.
func validateConfigRootDir(raw string) (string, error) {
	rootDir := strings.TrimSpace(raw)
	if rootDir == "" {
		return "", fmt.Errorf("invalid rootDir %q: must not be empty", raw)
	}
	if filepath.IsAbs(rootDir) || filepath.VolumeName(rootDir) != "" || strings.HasPrefix(rootDir, "/") || strings.HasPrefix(rootDir, `\`) {
		return "", fmt.Errorf("invalid rootDir %q: must be a relative path inside the project", rootDir)
	}
	cleaned := filepath.Clean(rootDir)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid rootDir %q: must not point outside the project", rootDir)
	}
	return rootDir, nil
}

// fileExists reports whether path exists as a regular file. Symlinks are
// resolved; a dangling symlink or a non-regular file (e.g. a directory) is an
// error, since writing through a dangling link would create a file at the
// link target, which may be outside the current project.
func fileExists(path string) (bool, error) {
	linfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check %s: %w", path, err)
	}
	info := linfo
	if linfo.Mode()&os.ModeSymlink != 0 {
		info, err = os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return false, fmt.Errorf("%s is a dangling symlink; remove it or fix its target before running config init", path)
			}
			return false, fmt.Errorf("failed to check %s: %w", path, err)
		}
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file", path)
	}
	return true, nil
}
