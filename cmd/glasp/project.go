package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/takihito/glasp/internal/config"
)

const maxTitleLength = 256

var scriptIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func ensureNoExistingClaspConfig(projectRoot string) error {
	if projectRoot == "" {
		return fmt.Errorf("project root is empty")
	}
	cfgPath := filepath.Join(projectRoot, ".clasp.json")
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to check .clasp.json: %w", err)
	}
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ScriptID) != "" {
		return fmt.Errorf(".clasp.json already exists in %s", projectRoot)
	}
	return nil
}

func validateTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", fmt.Errorf("title is required")
	}
	if utf8.RuneCountInString(trimmed) > maxTitleLength {
		return "", fmt.Errorf("title too long (max %d characters)", maxTitleLength)
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("title contains invalid control characters")
		}
	}
	return trimmed, nil
}

func validateScriptID(scriptID string) (string, error) {
	trimmed := strings.TrimSpace(scriptID)
	if trimmed == "" {
		return "", fmt.Errorf("script ID is required")
	}
	if !scriptIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid script ID format")
	}
	return trimmed, nil
}

func scriptIDFromConfig(projectRoot string) (string, error) {
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.ScriptID) == "" {
		return "", fmt.Errorf("script ID is required in .clasp.json")
	}
	return validateScriptID(cfg.ScriptID)
}

func validateCreateType(projectType string) error {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "standalone", "webapp", "api", "docs", "sheets", "slides", "forms":
		return nil
	default:
		return fmt.Errorf("invalid project type %q", projectType)
	}
}

// findExistingProjectRoot locates the nearest .clasp.json by searching upward
// from the current directory. Returns an error if no .clasp.json is found.
// When the project root differs from the current directory, the resolved path
// is printed to stdout so users know which directory glasp is operating from.
func findExistingProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine project root: %w", err)
	}
	root, err := config.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to determine project root: %w", err)
	}
	if root == "" {
		return "", fmt.Errorf(".clasp.json not found in current directory or any parent directory")
	}
	if filepath.Clean(root) != filepath.Clean(cwd) {
		fmt.Fprintf(stderr, "Project root: %s\n", root)
	}
	return root, nil
}

// projectContext bundles the resolved project root, its .clasp.json config,
// and the validated script ID shared by most remote commands.
type projectContext struct {
	Root     string
	Config   *config.ClaspConfig
	ScriptID string
}

// loadProjectContext resolves the nearest project root, loads .clasp.json,
// and validates its scriptId.
func loadProjectContext() (*projectContext, error) {
	root, err := findExistingProjectRoot()
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadClaspConfig(root)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ScriptID) == "" {
		return nil, fmt.Errorf("script ID is required in .clasp.json")
	}
	scriptID, err := validateScriptID(cfg.ScriptID)
	if err != nil {
		return nil, err
	}
	return &projectContext{Root: root, Config: cfg, ScriptID: scriptID}, nil
}

func optionalAuthPath(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("--auth path is empty")
	}
	return filepath.Clean(trimmed), nil
}
