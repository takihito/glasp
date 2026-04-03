package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// ClaspConfig represents the structure of .clasp.json
type ClaspConfig struct {
	ScriptID  string                     `json:"scriptId"`
	RootDir   string                     `json:"rootDir,omitempty"`
	ProjectID string                     `json:"projectId,omitempty"`
	ParentID  string                     `json:"parentId,omitempty"`
	Extra     map[string]json.RawMessage `json:"-"`
}

// claspConfigFileName is the name of the clasp configuration file.
const claspConfigFileName = ".clasp.json"

// FindProjectRoot locates the nearest directory containing a .clasp.json.
// When no project root is found, it returns an empty string and a nil error.
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return findClaspRoot(cwd)
}

func findClaspRoot(startDir string) (string, error) {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, claspConfigFileName)); err == nil {
			return dir, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check %s: %w", claspConfigFileName, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// LoadClaspConfig loads the .clasp.json file from the specified directory.
func LoadClaspConfig(dir string) (*ClaspConfig, error) {
	filePath := filepath.Join(dir, claspConfigFileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found in %s", claspConfigFileName, dir)
		}
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var cfg ClaspConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", filePath, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s for extra fields: %w", filePath, err)
	}
	delete(raw, "scriptId")
	delete(raw, "rootDir")
	delete(raw, "projectId")
	delete(raw, "parentId")
	if len(raw) > 0 {
		cfg.Extra = raw
	}
	return &cfg, nil
}

// SaveClaspConfig saves the ClaspConfig to a .clasp.json file in the specified directory.
func SaveClaspConfig(dir string, cfg *ClaspConfig) error {
	filePath := filepath.Join(dir, claspConfigFileName)
	payload := map[string]any{
		"scriptId": cfg.ScriptID,
	}
	if cfg.RootDir != "" {
		payload["rootDir"] = cfg.RootDir
	}
	if cfg.ProjectID != "" {
		payload["projectId"] = cfg.ProjectID
	}
	if cfg.ParentID != "" {
		payload["parentId"] = cfg.ParentID
	}
	for key, value := range cfg.Extra {
		payload[key] = value
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ClaspConfig: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filePath, err)
	} else {
		log.Printf("Created clasp config file: %s", filePath)
	}
	return nil
}

// ClaspIgnore represents the .claspignore patterns.
// It is a wrapper around gitignore.GitIgnore.
type ClaspIgnore struct {
	ignore *gitignore.GitIgnore
}

// NewClaspIgnore parses the .claspignore file from the specified directory.
func NewClaspIgnore(dir string) (*ClaspIgnore, error) {
	filePath := filepath.Join(dir, ".claspignore")
	defaultLines := []string{".glasp/", "node_modules/"}
	// go-gitignore expects the file to exist. If it doesn't, we just
	// return an "empty" ignore object that matches nothing.
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &ClaspIgnore{ignore: gitignore.CompileIgnoreLines(defaultLines...)}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open .claspignore file: %w", err)
	}
	defer file.Close()

	var lines []string
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read .claspignore file: %w", err)
		}
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimRight(line, "\r")
		if err == io.EOF {
			if line != "" {
				lines = append(lines, line)
			}
			break
		}
		lines = append(lines, line)
	}
	lines = append(lines, defaultLines...)
	ign := gitignore.CompileIgnoreLines(lines...)
	return &ClaspIgnore{ignore: ign}, nil
}

// Matches returns true if the given path matches any of the ignore patterns.
func (ci *ClaspIgnore) Matches(path string) bool {
	if ci.ignore == nil {
		return false
	}
	// The library requires a second argument, which is `isDir`.
	// For simplicity, we treat all paths as files (isDir=false).
	// This might have minor edge cases if a pattern is specifically for a directory
	// but is generally sufficient for `clasp`'s use case.
	return ci.ignore.MatchesPath(path)
}
