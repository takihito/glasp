// Package archive persists push/pull snapshots under
// .glasp/archive/<scriptId>/<push|pull>/<timestamp>/ and loads push
// payloads back for replay (push --history-id).
package archive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"

	"google.golang.org/api/script/v1"
)

// Manifest describes one archived push/pull run (manifest.json).
type Manifest struct {
	ScriptID      string              `json:"scriptId"`
	Direction     string              `json:"direction"`
	Timestamp     string              `json:"timestamp"`
	FileExtension string              `json:"fileExtension"`
	Convert       string              `json:"convert"`
	Status        string              `json:"status"`
	PayloadIndex  []PayloadIndexEntry `json:"payloadIndex,omitempty"`
}

// PayloadIndexEntry maps an archived payload file to its remote path and type.
type PayloadIndexEntry struct {
	Path       string `json:"path"`
	RemotePath string `json:"remotePath"`
	Type       string `json:"type"`
}

// PullRun archives a pull: the canonical remote content and the working
// (possibly TS-converted) content. It returns the archive root directory.
func PullRun(projectRoot, scriptID string, cfg *config.ClaspConfig, canonicalContent, workingContent *script.Content, fileExtension string, mode transform.Mode) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	if err := config.EnsureGlaspDir(projectRoot); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	archiveRoot := filepath.Join(projectRoot, ".glasp", "archive", scriptID, "pull", timestamp)
	if err := os.MkdirAll(archiveRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	manifestPath := filepath.Join(archiveRoot, "manifest.json")
	manifest := Manifest{
		ScriptID:      scriptID,
		Direction:     "pull",
		Timestamp:     timestamp,
		FileExtension: fileExtension,
		Convert:       mode.Label(),
		Status:        "failed",
	}
	if err := writeManifest(manifestPath, manifest); err != nil {
		return "", err
	}

	canonicalDir := filepath.Join(archiveRoot, "canonical")
	workingDir := filepath.Join(archiveRoot, "working")
	if err := os.MkdirAll(canonicalDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create canonical archive directory: %w", err)
	}
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create working archive directory: %w", err)
	}

	canonicalOpts, err := syncer.OptionsFromConfig(canonicalDir, cfg, nil)
	if err != nil {
		return "", err
	}
	canonicalOpts.FileExtensions = syncer.DefaultFileExtensions()
	if _, err := syncer.ApplyRemoteContent(canonicalOpts, canonicalContent); err != nil {
		return "", err
	}

	workingOpts, err := syncer.OptionsFromConfig(workingDir, cfg, nil)
	if err != nil {
		return "", err
	}
	if _, err := syncer.ApplyRemoteContent(workingOpts, workingContent); err != nil {
		return "", err
	}

	manifest.Status = "success"
	if err := writeManifest(manifestPath, manifest); err != nil {
		return "", err
	}
	return archiveRoot, nil
}

// PushRun archives a push: the working files as collected locally and the
// payload files actually sent to the API. It returns the archive root directory.
func PushRun(projectRoot, scriptID string, workingFiles, payloadFiles []syncer.ProjectFile, fileExtension string, mode transform.Mode) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	if err := config.EnsureGlaspDir(projectRoot); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	archiveRoot := filepath.Join(projectRoot, ".glasp", "archive", scriptID, "push", timestamp)
	if err := os.MkdirAll(archiveRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	manifestPath := filepath.Join(archiveRoot, "manifest.json")
	manifest := Manifest{
		ScriptID:      scriptID,
		Direction:     "push",
		Timestamp:     timestamp,
		FileExtension: fileExtension,
		Convert:       mode.Label(),
		Status:        "failed",
		PayloadIndex:  buildPayloadIndex(payloadFiles),
	}
	if err := writeManifest(manifestPath, manifest); err != nil {
		return "", err
	}

	workingDir := filepath.Join(archiveRoot, "working")
	payloadDir := filepath.Join(archiveRoot, "payload")
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create working archive directory: %w", err)
	}
	if err := os.MkdirAll(payloadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create payload archive directory: %w", err)
	}
	if err := syncer.ArchiveLocalFiles(workingDir, workingFiles); err != nil {
		return "", err
	}
	if err := syncer.ArchiveLocalFiles(payloadDir, payloadFiles); err != nil {
		return "", err
	}

	manifest.Status = "success"
	if err := writeManifest(manifestPath, manifest); err != nil {
		return "", err
	}
	return archiveRoot, nil
}

func writeManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}
	return nil
}

// LoadPushPayload reads a push archive and returns its manifest and the
// payload files ready to push again. rootDir is the project rootDir used to
// strip legacy path prefixes when the manifest has no payload index.
func LoadPushPayload(archivePath, rootDir string) (Manifest, []syncer.ProjectFile, error) {
	manifestPath := filepath.Join(archivePath, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, nil, fmt.Errorf("archive manifest not found: %s", manifestPath)
		}
		return Manifest{}, nil, fmt.Errorf("failed to read archive manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Manifest{}, nil, fmt.Errorf("failed to parse archive manifest: %w", err)
	}
	if strings.TrimSpace(manifest.Direction) != "push" {
		return Manifest{}, nil, fmt.Errorf("archive direction must be push, got %q", manifest.Direction)
	}
	payloadDir := filepath.Join(archivePath, "payload")
	info, err := os.Stat(payloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, nil, fmt.Errorf("archive payload directory not found: %s", payloadDir)
		}
		return Manifest{}, nil, fmt.Errorf("failed to stat archive payload directory: %w", err)
	}
	if !info.IsDir() {
		return Manifest{}, nil, fmt.Errorf("archive payload path is not a directory: %s", payloadDir)
	}

	indexByPath := make(map[string]PayloadIndexEntry, len(manifest.PayloadIndex))
	for _, item := range manifest.PayloadIndex {
		key := normalizePayloadPath(item.Path)
		if key == "" {
			continue
		}
		indexByPath[key] = item
	}
	files := make([]syncer.ProjectFile, 0, 8)
	if err := filepath.Walk(payloadDir, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(payloadDir, currentPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		fileType, ok := payloadFileType(rel)
		if !ok {
			return nil
		}
		source, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}
		remotePath := ""
		if idx, ok := indexByPath[normalizePayloadPath(rel)]; ok &&
			strings.TrimSpace(idx.RemotePath) != "" &&
			(strings.TrimSpace(idx.Type) == "" || strings.TrimSpace(idx.Type) == fileType) {
			remotePath = strings.TrimSpace(idx.RemotePath)
		} else {
			remotePath = payloadRemotePath(rel, fileType, rootDir)
		}
		if strings.TrimSpace(remotePath) == "" {
			return fmt.Errorf("failed to determine remote path for payload file: %s", rel)
		}
		files = append(files, syncer.ProjectFile{
			LocalPath:  rel,
			RemotePath: remotePath,
			Type:       fileType,
			Source:     string(source),
		})
		return nil
	}); err != nil {
		return Manifest{}, nil, fmt.Errorf("failed to read archive payload files: %w", err)
	}
	if len(files) == 0 {
		return Manifest{}, nil, fmt.Errorf("archive payload has no pushable files: %s", payloadDir)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].LocalPath < files[j].LocalPath
	})
	return manifest, files, nil
}

func payloadFileType(relPath string) (string, bool) {
	clean := filepath.ToSlash(relPath)
	lower := strings.ToLower(clean)
	if filepath.Base(clean) == "appsscript.json" {
		return syncer.FileTypeJSON, true
	}
	switch filepath.Ext(lower) {
	case ".js", ".gs":
		return syncer.FileTypeServerJS, true
	case ".html":
		return syncer.FileTypeHTML, true
	default:
		return "", false
	}
}

func payloadRemotePath(relPath, fileType, rootDir string) string {
	clean := normalizePayloadPath(relPath)
	rootPrefix := normalizeRootDirPrefix(rootDir)
	if rootPrefix != "" {
		prefix := rootPrefix + "/"
		if strings.HasPrefix(clean, prefix) {
			clean = strings.TrimPrefix(clean, prefix)
		}
	}
	if fileType == syncer.FileTypeJSON && filepath.Base(clean) == "appsscript.json" {
		return "appsscript"
	}
	ext := filepath.Ext(clean)
	if ext == "" {
		return clean
	}
	return strings.TrimSuffix(clean, ext)
}

func normalizeRootDirPrefix(rootDir string) string {
	clean := strings.TrimSpace(rootDir)
	if clean == "" {
		return ""
	}
	clean = filepath.ToSlash(filepath.Clean(clean))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	clean = strings.TrimSuffix(clean, "/")
	if clean == "." {
		return ""
	}
	return clean
}

func normalizePayloadPath(path string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	return clean
}

func buildPayloadIndex(payloadFiles []syncer.ProjectFile) []PayloadIndexEntry {
	index := make([]PayloadIndexEntry, 0, len(payloadFiles))
	for _, file := range payloadFiles {
		path := normalizePayloadPath(file.LocalPath)
		remotePath := strings.TrimSpace(file.RemotePath)
		fileType := strings.TrimSpace(file.Type)
		if path == "" || remotePath == "" || fileType == "" {
			continue
		}
		index = append(index, PayloadIndexEntry{
			Path:       path,
			RemotePath: remotePath,
			Type:       fileType,
		})
	}
	sort.Slice(index, func(i, j int) bool {
		return index[i].Path < index[j].Path
	})
	return index
}
