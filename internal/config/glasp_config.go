package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	glaspConfigDirName  = ".glasp"
	glaspConfigFileName = "config.json"
)

// EnsureGlaspDir creates the .glasp/ directory under projectRoot if it does
// not already exist. When the directory is newly created it also ensures that
// .claspignore contains a ".glasp/" entry so that clasp will not push glasp's
// internal files.
func EnsureGlaspDir(projectRoot string) error {
	dir := filepath.Join(projectRoot, glaspConfigDirName)
	created := false
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		created = true
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	if created {
		if err := ensureClaspIgnoreEntry(projectRoot, glaspConfigDirName+"/"); err != nil {
			return err
		}
	}
	return nil
}

// ensureClaspIgnoreEntry appends entry to .claspignore if the file does not
// already contain it. If .claspignore does not exist, it is created.
func ensureClaspIgnoreEntry(projectRoot, entry string) error {
	filePath := filepath.Join(projectRoot, ".claspignore")

	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read .claspignore: %w", err)
	}
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == entry {
				return nil // already present
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			return fmt.Errorf("failed to scan .claspignore: %w", scanErr)
		}
	}

	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .claspignore for writing: %w", err)
	}
	defer f.Close()

	// If the file already has content and doesn't end with a newline, add one.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to .claspignore: %w", err)
		}
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("failed to write to .claspignore: %w", err)
	}
	return nil
}

// GlaspConfig represents the structure of .glasp/config.json.
type GlaspConfig struct {
	Archive ArchiveConfig `json:"archive"`
}

// ArchiveConfig controls archive settings.
type ArchiveConfig struct {
	Pull bool `json:"pull"`
	Push bool `json:"push"`
}

// LoadGlaspConfig loads .glasp/config.json. If the file does not exist, it returns a default config.
func LoadGlaspConfig(projectRoot string) (*GlaspConfig, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf("project root is empty")
	}
	filePath := filepath.Join(projectRoot, glaspConfigDirName, glaspConfigFileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlaspConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}
	var cfg GlaspConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", filePath, err)
	}
	return &cfg, nil
}

// SaveGlaspConfig saves .glasp/config.json in the given project root.
func SaveGlaspConfig(projectRoot string, cfg *GlaspConfig) error {
	if projectRoot == "" {
		return fmt.Errorf("project root is empty")
	}
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if err := EnsureGlaspDir(projectRoot); err != nil {
		return err
	}
	filePath := filepath.Join(projectRoot, glaspConfigDirName, glaspConfigFileName)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal glasp config: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filePath, err)
	}
	return nil
}
