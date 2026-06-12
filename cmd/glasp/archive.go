package main

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

type archiveManifest struct {
	ScriptID      string                     `json:"scriptId"`
	Direction     string                     `json:"direction"`
	Timestamp     string                     `json:"timestamp"`
	FileExtension string                     `json:"fileExtension"`
	Convert       string                     `json:"convert"`
	Status        string                     `json:"status"`
	PayloadIndex  []archivePayloadIndexEntry `json:"payloadIndex,omitempty"`
}

type archivePayloadIndexEntry struct {
	Path       string `json:"path"`
	RemotePath string `json:"remotePath"`
	Type       string `json:"type"`
}

func archivePullRun(projectRoot, scriptID string, cfg *config.ClaspConfig, canonicalContent, workingContent *script.Content, fileExtension string, mode transform.Mode) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	if err := config.EnsureGlaspDir(projectRoot); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	archiveRoot := filepath.Join(projectRoot, ".glasp", "archive", scriptID, "pull", timestamp)
	if err := os.MkdirAll(archiveRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	manifestPath := filepath.Join(archiveRoot, "manifest.json")
	manifest := archiveManifest{
		ScriptID:      scriptID,
		Direction:     "pull",
		Timestamp:     timestamp,
		FileExtension: fileExtension,
		Convert:       convertLabel(mode),
		Status:        "failed",
	}
	if err := writeArchiveManifest(manifestPath, manifest); err != nil {
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
	if err := writeArchiveManifest(manifestPath, manifest); err != nil {
		return "", err
	}
	return archiveRoot, nil
}

func archivePushRun(projectRoot, scriptID string, workingFiles, payloadFiles []syncer.ProjectFile, fileExtension string, mode transform.Mode) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	if err := config.EnsureGlaspDir(projectRoot); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	archiveRoot := filepath.Join(projectRoot, ".glasp", "archive", scriptID, "push", timestamp)
	if err := os.MkdirAll(archiveRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}
	manifestPath := filepath.Join(archiveRoot, "manifest.json")
	manifest := archiveManifest{
		ScriptID:      scriptID,
		Direction:     "push",
		Timestamp:     timestamp,
		FileExtension: fileExtension,
		Convert:       convertLabel(mode),
		Status:        "failed",
		PayloadIndex:  buildArchivePayloadIndex(payloadFiles),
	}
	if err := writeArchiveManifest(manifestPath, manifest); err != nil {
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
	if err := writeArchiveManifest(manifestPath, manifest); err != nil {
		return "", err
	}
	return archiveRoot, nil
}

func writeArchiveManifest(path string, manifest archiveManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}
	return nil
}

func loadPushArchivePayload(archivePath, rootDir string) (archiveManifest, []syncer.ProjectFile, error) {
	manifestPath := filepath.Join(archivePath, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return archiveManifest{}, nil, fmt.Errorf("archive manifest not found: %s", manifestPath)
		}
		return archiveManifest{}, nil, fmt.Errorf("failed to read archive manifest: %w", err)
	}
	var manifest archiveManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return archiveManifest{}, nil, fmt.Errorf("failed to parse archive manifest: %w", err)
	}
	if strings.TrimSpace(manifest.Direction) != "push" {
		return archiveManifest{}, nil, fmt.Errorf("archive direction must be push, got %q", manifest.Direction)
	}
	payloadDir := filepath.Join(archivePath, "payload")
	info, err := os.Stat(payloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return archiveManifest{}, nil, fmt.Errorf("archive payload directory not found: %s", payloadDir)
		}
		return archiveManifest{}, nil, fmt.Errorf("failed to stat archive payload directory: %w", err)
	}
	if !info.IsDir() {
		return archiveManifest{}, nil, fmt.Errorf("archive payload path is not a directory: %s", payloadDir)
	}

	indexByPath := make(map[string]archivePayloadIndexEntry, len(manifest.PayloadIndex))
	for _, item := range manifest.PayloadIndex {
		key := normalizeArchivePayloadPath(item.Path)
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
		fileType, ok := archivePayloadFileType(rel)
		if !ok {
			return nil
		}
		source, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}
		remotePath := ""
		if idx, ok := indexByPath[normalizeArchivePayloadPath(rel)]; ok &&
			strings.TrimSpace(idx.RemotePath) != "" &&
			(strings.TrimSpace(idx.Type) == "" || strings.TrimSpace(idx.Type) == fileType) {
			remotePath = strings.TrimSpace(idx.RemotePath)
		} else {
			remotePath = archivePayloadRemotePath(rel, fileType, rootDir)
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
		return archiveManifest{}, nil, fmt.Errorf("failed to read archive payload files: %w", err)
	}
	if len(files) == 0 {
		return archiveManifest{}, nil, fmt.Errorf("archive payload has no pushable files: %s", payloadDir)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].LocalPath < files[j].LocalPath
	})
	return manifest, files, nil
}

func archivePayloadFileType(relPath string) (string, bool) {
	clean := filepath.ToSlash(relPath)
	lower := strings.ToLower(clean)
	if filepath.Base(clean) == "appsscript.json" {
		return "JSON", true
	}
	switch filepath.Ext(lower) {
	case ".js", ".gs":
		return "SERVER_JS", true
	case ".html":
		return "HTML", true
	default:
		return "", false
	}
}

func archivePayloadRemotePath(relPath, fileType, rootDir string) string {
	clean := normalizeArchivePayloadPath(relPath)
	rootPrefix := normalizeRootDirPrefix(rootDir)
	if rootPrefix != "" {
		prefix := rootPrefix + "/"
		if strings.HasPrefix(clean, prefix) {
			clean = strings.TrimPrefix(clean, prefix)
		}
	}
	if fileType == "JSON" && filepath.Base(clean) == "appsscript.json" {
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

func normalizeArchivePayloadPath(path string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	return clean
}

func buildArchivePayloadIndex(payloadFiles []syncer.ProjectFile) []archivePayloadIndexEntry {
	index := make([]archivePayloadIndexEntry, 0, len(payloadFiles))
	for _, file := range payloadFiles {
		path := normalizeArchivePayloadPath(file.LocalPath)
		remotePath := strings.TrimSpace(file.RemotePath)
		fileType := strings.TrimSpace(file.Type)
		if path == "" || remotePath == "" || fileType == "" {
			continue
		}
		index = append(index, archivePayloadIndexEntry{
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

func modeFromArchiveConvert(label string) transform.Mode {
	switch strings.TrimSpace(label) {
	case "gas-to-ts":
		return transform.ModeGasToTS
	case "ts-to-gas":
		return transform.ModeTSToGas
	default:
		return transform.Mode("")
	}
}
