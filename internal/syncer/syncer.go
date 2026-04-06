package syncer

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/takihito/glasp/internal/config"
	"google.golang.org/api/script/v1"
)

const (
	fileTypeServerJS = "SERVER_JS"
	fileTypeHTML     = "HTML"
	fileTypeJSON     = "JSON"
)

// Options defines settings for local/remote synchronization.
type Options struct {
	ProjectRoot        string
	RootDir            string
	Ignore             *config.ClaspIgnore
	FileExtensions     map[string][]string
	FilePushOrder      []string
	SkipSubdirectories bool
}

// ProjectFile represents a file mapped between local and remote.
type ProjectFile struct {
	// LocalPath is the project-relative path on disk (e.g. "src/Code.gs").
	LocalPath string
	// RemotePath is the Apps Script file name/path without extension (e.g. "Code" or "ui/page").
	RemotePath string
	// Type maps to Apps Script file types (SERVER_JS, HTML, JSON).
	Type string
	// Source is the file contents to send to or received from the Script API.
	Source string
}

// OptionsFromConfig builds Options from a .clasp.json config.
func OptionsFromConfig(projectRoot string, cfg *config.ClaspConfig, ignore *config.ClaspIgnore) (Options, error) {
	if cfg == nil {
		return Options{}, fmt.Errorf("config is nil")
	}
	rootDir := strings.TrimSpace(cfg.RootDir)
	if rootDir == "" && cfg.Extra != nil {
		if srcDir, ok, err := parseString(cfg.Extra, "srcDir"); err != nil {
			return Options{}, err
		} else if ok {
			rootDir = srcDir
		}
	}
	fileExtensions, err := parseFileExtensions(cfg)
	if err != nil {
		return Options{}, err
	}
	filePushOrder, err := parseFilePushOrder(cfg)
	if err != nil {
		return Options{}, err
	}
	skipSubdirectories := false
	if cfg.Extra != nil {
		if skip, ok, err := parseBool(cfg.Extra, "ignoreSubdirectories"); err != nil {
			return Options{}, err
		} else if ok {
			skipSubdirectories = skip
		}
	}

	return Options{
		ProjectRoot:        projectRoot,
		RootDir:            rootDir,
		Ignore:             ignore,
		FileExtensions:     fileExtensions,
		FilePushOrder:      filePushOrder,
		SkipSubdirectories: skipSubdirectories,
	}, nil
}

// DefaultFileExtensions returns the default Apps Script file extensions.
func DefaultFileExtensions() map[string][]string {
	return map[string][]string{
		fileTypeServerJS: {".js", ".gs", ".ts"},
		fileTypeHTML:     {".html"},
		fileTypeJSON:     {".json"},
	}
}

// CollectLocalFiles scans local files and returns push-ready project files.
func CollectLocalFiles(opts Options) ([]ProjectFile, error) {
	contentDir, err := contentDir(opts)
	if err != nil {
		return nil, err
	}
	fileExtensions := normalizeFileExtensions(opts.FileExtensions)
	sameContentAndProjectRoot := filepath.Clean(contentDir) == filepath.Clean(opts.ProjectRoot)

	conflicts := make(map[string]ProjectFile)
	var files []ProjectFile
	if opts.ProjectRoot == "" {
		return nil, fmt.Errorf("project root is empty")
	}

	// Backward compatibility: if rootDir != projectRoot and rootDir has no appsscript.json,
	// accept legacy project-root appsscript.json.
	if !sameContentAndProjectRoot {
		contentAppsscript := filepath.Join(contentDir, "appsscript.json")
		_, contentErr := os.Stat(contentAppsscript)
		if contentErr != nil && !os.IsNotExist(contentErr) {
			return nil, contentErr
		}
		if os.IsNotExist(contentErr) {
			rootAppsscript := filepath.Join(opts.ProjectRoot, "appsscript.json")
			if _, err := os.Stat(rootAppsscript); err == nil {
				source, err := os.ReadFile(rootAppsscript)
				if err != nil {
					return nil, err
				}
				files = append(files, ProjectFile{
					LocalPath:  "appsscript.json",
					RemotePath: "appsscript",
					Type:       fileTypeJSON,
					Source:     string(source),
				})
				conflicts["appsscript"] = ProjectFile{
					LocalPath: "appsscript.json",
					Type:      fileTypeJSON,
				}
			} else if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		}
	}
	walkFn := func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			// Always skip .glasp/ directory — it contains internal data
			// (tokens, archives, config) that must never be pushed.
			if entry.Name() == ".glasp" {
				return filepath.SkipDir
			}
			if opts.SkipSubdirectories && currentPath != contentDir {
				return filepath.SkipDir
			}
			return nil
		}
		relToRoot, err := filepath.Rel(opts.ProjectRoot, currentPath)
		if err != nil {
			return err
		}
		relToRoot = filepath.ToSlash(relToRoot)
		if opts.Ignore != nil && opts.Ignore.Matches(relToRoot) {
			return nil
		}
		// Skip TypeScript declaration files (.d.ts) — they are not deployable.
		if strings.HasSuffix(strings.ToLower(relToRoot), ".d.ts") {
			return nil
		}
		fileType := fileTypeForPath(relToRoot, fileExtensions)
		if fileType == "" {
			return nil
		}
		relToContent, err := filepath.Rel(contentDir, currentPath)
		if err != nil {
			return err
		}
		relToContent = filepath.ToSlash(relToContent)
		remotePath := remotePathFromLocal(relToContent, fileType)
		if existing, exists := conflicts[remotePath]; exists {
			return fmt.Errorf("conflicting remote file name: %s (%s:%s vs %s:%s)",
				remotePath,
				existing.LocalPath,
				existing.Type,
				relToRoot,
				fileType,
			)
		}
		conflicts[remotePath] = ProjectFile{
			LocalPath: relToRoot,
			Type:      fileType,
		}
		source, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}
		files = append(files, ProjectFile{
			LocalPath:  relToRoot,
			RemotePath: remotePath,
			Type:       fileType,
			Source:     string(source),
		})
		return nil
	}
	if err := filepath.WalkDir(contentDir, walkFn); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].LocalPath < files[j].LocalPath
	})
	return files, nil
}

// SortFilesByPushOrder sorts files using filePushOrder rules.
func SortFilesByPushOrder(files []ProjectFile, filePushOrder []string, rootDir string) {
	if len(filePushOrder) == 0 {
		return
	}
	normalizedOrder := make([]string, 0, len(filePushOrder))
	for _, entry := range filePushOrder {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		entry = filepath.ToSlash(entry)
		entry = strings.TrimPrefix(entry, "./")
		normalizedOrder = append(normalizedOrder, entry)
	}
	orderIndex := make(map[string]int, len(normalizedOrder))
	for idx, entry := range normalizedOrder {
		if _, exists := orderIndex[entry]; !exists {
			orderIndex[entry] = idx
		}
	}
	rootPrefix := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(rootDir)), "/")

	sort.Slice(files, func(i, j int) bool {
		left := files[i].LocalPath
		right := files[j].LocalPath

		leftIndex := pushOrderIndex(orderIndex, left, rootPrefix)
		rightIndex := pushOrderIndex(orderIndex, right, rootPrefix)

		if leftIndex == -1 && rightIndex == -1 {
			return left < right
		}
		if leftIndex == -1 {
			return false
		}
		if rightIndex == -1 {
			return true
		}
		return leftIndex < rightIndex
	})
}

// BuildContent converts local files to a Script API content payload.
func BuildContent(files []ProjectFile) *script.Content {
	scriptFiles := make([]*script.File, 0, len(files))
	for _, file := range files {
		scriptFiles = append(scriptFiles, &script.File{
			Name:   file.RemotePath,
			Type:   file.Type,
			Source: file.Source,
		})
	}
	return &script.Content{Files: scriptFiles}
}

// ArchiveLocalFiles writes local files to the given archive root.
func ArchiveLocalFiles(archiveRoot string, files []ProjectFile) error {
	if archiveRoot == "" {
		return fmt.Errorf("archive root is empty")
	}
	for _, file := range files {
		targetPath, err := archiveTargetPath(archiveRoot, file.LocalPath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create archive directory: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(file.Source), 0644); err != nil {
			return fmt.Errorf("failed to write archive file: %w", err)
		}
	}
	return nil
}

// ApplyRemoteContent writes remote content to the local filesystem.
func ApplyRemoteContent(opts Options, content *script.Content) ([]ProjectFile, error) {
	if content == nil {
		return nil, fmt.Errorf("content is nil")
	}
	contentDir, err := contentDir(opts)
	if err != nil {
		return nil, err
	}
	fileExtensions := normalizeFileExtensions(opts.FileExtensions)

	var written []ProjectFile
	for _, file := range content.Files {
		if file == nil {
			continue
		}
		if file.Name == "" {
			return nil, fmt.Errorf("remote file name is empty")
		}
		ext, err := extensionForType(file.Type, fileExtensions)
		if err != nil {
			return nil, err
		}
		if strings.Contains(file.Name, "\x00") {
			return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
		}
		if strings.HasPrefix(file.Name, `\\`) {
			return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
		}
		rawSlashed := filepath.ToSlash(file.Name)
		if strings.HasPrefix(rawSlashed, "/") {
			return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
		}
		if len(rawSlashed) >= 2 && rawSlashed[1] == ':' {
			drive := rawSlashed[0]
			if (drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z') {
				return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
			}
		}
		for _, part := range strings.Split(rawSlashed, "/") {
			if part == ".." {
				return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
			}
		}
		remoteName := path.Clean(rawSlashed)
		if remoteName == "." || strings.HasPrefix(remoteName, "../") {
			return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
		}
		localRel := remoteName + ext
		targetPath := filepath.Join(contentDir, filepath.FromSlash(localRel))
		checkBase := contentDir
		relToContent, err := filepath.Rel(checkBase, targetPath)
		if err != nil {
			return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
		}
		relToContent = filepath.ToSlash(relToContent)
		if relToContent == ".." || strings.HasPrefix(relToContent, "../") {
			return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, err
		}
		source := file.Source
		if err := os.WriteFile(targetPath, []byte(source), 0644); err != nil {
			return nil, err
		}
		relToRoot, err := filepath.Rel(opts.ProjectRoot, targetPath)
		if err != nil {
			return nil, err
		}
		written = append(written, ProjectFile{
			LocalPath:  filepath.ToSlash(relToRoot),
			RemotePath: remoteName,
			Type:       file.Type,
			Source:     source,
		})
	}
	sort.Slice(written, func(i, j int) bool {
		return written[i].LocalPath < written[j].LocalPath
	})
	return written, nil
}

func contentDir(opts Options) (string, error) {
	if opts.ProjectRoot == "" {
		return "", fmt.Errorf("project root is empty")
	}
	rootDir := strings.TrimSpace(opts.RootDir)
	if rootDir == "" {
		rootDir = "."
	}
	return filepath.Join(opts.ProjectRoot, filepath.Clean(rootDir)), nil
}

func fileTypeForPath(localPath string, fileExtensions map[string][]string) string {
	ext := strings.ToLower(path.Ext(localPath))
	if matchesExtension(ext, fileExtensions[fileTypeServerJS]) {
		return fileTypeServerJS
	}
	if matchesExtension(ext, fileExtensions[fileTypeHTML]) {
		return fileTypeHTML
	}
	if matchesExtension(ext, fileExtensions[fileTypeJSON]) && path.Base(localPath) == "appsscript.json" {
		return fileTypeJSON
	}
	return ""
}

func remotePathFromLocal(relToContent string, fileType string) string {
	dir := path.Dir(relToContent)
	base := path.Base(relToContent)
	name := strings.TrimSuffix(base, path.Ext(base))
	remotePath := name
	if dir != "." {
		remotePath = path.Join(dir, name)
	}
	if fileType == fileTypeJSON && path.Base(relToContent) == "appsscript.json" {
		remotePath = "appsscript"
	}
	return remotePath
}

func extensionForType(fileType string, fileExtensions map[string][]string) (string, error) {
	switch fileType {
	case fileTypeServerJS:
		return firstExtension(fileExtensions[fileTypeServerJS]), nil
	case fileTypeHTML:
		return firstExtension(fileExtensions[fileTypeHTML]), nil
	case fileTypeJSON:
		return firstExtension(fileExtensions[fileTypeJSON]), nil
	default:
		return "", fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func firstExtension(extensions []string) string {
	if len(extensions) == 0 {
		return ""
	}
	return extensions[0]
}

func matchesExtension(ext string, extensions []string) bool {
	for _, item := range extensions {
		if item == ext {
			return true
		}
	}
	return false
}

func normalizeFileExtensions(input map[string][]string) map[string][]string {
	base := DefaultFileExtensions()
	if input == nil {
		return base
	}
	for key, values := range input {
		normalized := normalizeExtensions(values)
		if len(normalized) == 0 {
			continue
		}
		base[key] = normalized
	}
	return base
}

func normalizeExtensions(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, ext := range values {
		ext = strings.TrimSpace(strings.ToLower(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalized = append(normalized, ext)
	}
	return normalized
}

func parseFileExtensions(cfg *config.ClaspConfig) (map[string][]string, error) {
	scriptExtensions := []string{"js", "gs", "ts"}
	htmlExtensions := []string{"html"}
	jsonExtensions := []string{"json"}

	if cfg != nil && cfg.Extra != nil {
		if fileExt, ok, err := parseString(cfg.Extra, "fileExtension"); err != nil {
			return nil, err
		} else if ok {
			// Start from the configured extension, then ensure "ts" is
			// always present so .ts files are collected for auto-transpile.
			seen := map[string]bool{fileExt: true}
			exts := []string{fileExt}
			if !seen["ts"] {
				exts = append(exts, "ts")
			}
			scriptExtensions = exts
		}
		if scriptExts, ok, err := parseStringSlice(cfg.Extra, "scriptExtensions"); err != nil {
			return nil, err
		} else if ok {
			scriptExtensions = scriptExts
		}
		if htmlExts, ok, err := parseStringSlice(cfg.Extra, "htmlExtensions"); err != nil {
			return nil, err
		} else if ok {
			htmlExtensions = htmlExts
		}
		if jsonExts, ok, err := parseStringSlice(cfg.Extra, "jsonExtensions"); err != nil {
			return nil, err
		} else if ok {
			jsonExtensions = jsonExts
		}
	}

	return map[string][]string{
		fileTypeServerJS: normalizeExtensions(scriptExtensions),
		fileTypeHTML:     normalizeExtensions(htmlExtensions),
		fileTypeJSON:     normalizeExtensions(jsonExtensions),
	}, nil
}

func parseFilePushOrder(cfg *config.ClaspConfig) ([]string, error) {
	if cfg == nil || cfg.Extra == nil {
		return nil, nil
	}
	if raw, ok := cfg.Extra["filePushOrder"]; ok {
		var order []string
		if err := json.Unmarshal(raw, &order); err != nil {
			return nil, fmt.Errorf("invalid filePushOrder: %w", err)
		}
		return order, nil
	}
	return nil, nil
}

func parseString(extra map[string]json.RawMessage, key string) (string, bool, error) {
	raw, ok := extra[key]
	if !ok {
		return "", false, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return value, true, nil
}

func parseStringSlice(extra map[string]json.RawMessage, key string) ([]string, bool, error) {
	raw, ok := extra[key]
	if !ok {
		return nil, false, nil
	}
	var value []string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return value, true, nil
}

func parseBool(extra map[string]json.RawMessage, key string) (bool, bool, error) {
	raw, ok := extra[key]
	if !ok {
		return false, false, nil
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return value, true, nil
}

func pushOrderIndex(order map[string]int, localPath, rootPrefix string) int {
	if index, ok := order[localPath]; ok {
		return index
	}
	if rootPrefix == "." || rootPrefix == "" {
		return -1
	}
	prefix := rootPrefix + "/"
	if strings.HasPrefix(localPath, prefix) {
		trimmed := strings.TrimPrefix(localPath, prefix)
		if index, ok := order[trimmed]; ok {
			return index
		}
	}
	return -1
}

func archiveTargetPath(archiveRoot, localPath string) (string, error) {
	if archiveRoot == "" {
		return "", fmt.Errorf("archive root is empty")
	}
	if strings.TrimSpace(localPath) == "" {
		return "", fmt.Errorf("local path is empty")
	}
	if strings.Contains(localPath, "\x00") {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	slashed := filepath.ToSlash(localPath)
	for _, part := range strings.Split(slashed, "/") {
		if part == ".." {
			return "", fmt.Errorf("invalid local path: %s", localPath)
		}
	}
	if strings.HasPrefix(slashed, "/") {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	if strings.HasPrefix(slashed, `\\`) {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	if len(slashed) >= 2 && slashed[1] == ':' {
		drive := slashed[0]
		if (drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z') {
			return "", fmt.Errorf("invalid local path: %s", localPath)
		}
	}
	cleaned := filepath.Clean(filepath.FromSlash(slashed))
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	targetPath := filepath.Join(archiveRoot, cleaned)
	relToRoot, err := filepath.Rel(archiveRoot, targetPath)
	if err != nil {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	relToRoot = filepath.ToSlash(relToRoot)
	if relToRoot == ".." || strings.HasPrefix(relToRoot, "../") {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	return targetPath, nil
}
