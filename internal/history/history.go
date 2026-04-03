package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"glasp/internal/config"
)

const (
	glaspDirName    = ".glasp"
	historyFileName = "history.jsonl"
	lockFileName    = "history.lock"
)

var appendMu sync.Mutex

type Archive struct {
	Enabled   bool   `json:"enabled"`
	Direction string `json:"direction"`
	Path      string `json:"path"`
}

type Entry struct {
	ID         int64    `json:"id"`
	Timestamp  string   `json:"timestamp"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	Status     string   `json:"status"`
	Error      string   `json:"error"`
	DurationMs int64    `json:"durationMs"`
	Archive    Archive  `json:"archive"`
}

type ReadOptions struct {
	Limit   int
	Status  string
	Command string
	Order   string
}

func FilePath(projectRoot string) string {
	return filepath.Join(projectRoot, glaspDirName, historyFileName)
}

func Append(projectRoot string, entry Entry) error {
	if projectRoot == "" {
		return fmt.Errorf("project root is empty")
	}
	if err := config.EnsureGlaspDir(projectRoot); err != nil {
		return fmt.Errorf("failed to ensure .glasp dir: %w", err)
	}
	historyPath := FilePath(projectRoot)
	appendMu.Lock()
	defer appendMu.Unlock()

	lockPath := filepath.Join(projectRoot, glaspDirName, lockFileName)
	lockFile, err := acquireFileLock(lockPath)
	if err != nil {
		return fmt.Errorf("failed to acquire history lock: %w", err)
	}
	defer releaseFileLock(lockFile)

	maxID, err := maxIDFromFile(historyPath)
	if err != nil {
		return err
	}
	if entry.ID <= 0 {
		entry.ID = maxID + 1
	}
	if strings.TrimSpace(entry.Timestamp) == "" {
		entry.Timestamp = time.Now().Format(time.RFC3339)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal history entry: %w", err)
	}
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to append history entry: %w", err)
	}
	return nil
}

func Read(projectRoot string, opts ReadOptions) ([]Entry, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf("project root is empty")
	}
	if opts.Limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "success" && status != "error" {
		return nil, fmt.Errorf("invalid status %q", opts.Status)
	}
	order := strings.TrimSpace(opts.Order)
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		return nil, fmt.Errorf("invalid order %q", opts.Order)
	}

	historyPath := FilePath(projectRoot)
	entries, err := readAll(historyPath)
	if err != nil {
		return nil, err
	}

	command := strings.TrimSpace(opts.Command)
	filtered := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if status != "all" && entry.Status != status {
			continue
		}
		if command != "" && entry.Command != command {
			continue
		}
		filtered = append(filtered, entry)
	}

	if order == "desc" {
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered, nil
}

func GetByID(projectRoot string, id int64) (Entry, bool, error) {
	if projectRoot == "" {
		return Entry{}, false, fmt.Errorf("project root is empty")
	}
	if id <= 0 {
		return Entry{}, false, fmt.Errorf("id must be > 0")
	}
	historyPath := FilePath(projectRoot)
	entries, err := readAll(historyPath)
	if err != nil {
		return Entry{}, false, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true, nil
		}
	}
	return Entry{}, false, nil
}

func maxIDFromFile(historyPath string) (int64, error) {
	entries, err := readAll(historyPath)
	if err != nil {
		return 0, err
	}
	var maxID int64
	for _, entry := range entries {
		if entry.ID > maxID {
			maxID = entry.ID
		}
	}
	return maxID, nil
}

func readAll(historyPath string) ([]Entry, error) {
	f, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal history line %d: %w", lineNo, err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}
	return entries, nil
}
