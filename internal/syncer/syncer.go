package syncer

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/fsutil"
	"google.golang.org/api/script/v1"
)

// Apps Script file types as used by the Script API.
const (
	FileTypeServerJS = "SERVER_JS"
	FileTypeHTML     = "HTML"
	FileTypeJSON     = "JSON"
)

// Options defines settings for local/remote synchronization.
type Options struct {
	ProjectRoot        string
	RootDir            string
	Ignore             *config.ClaspIgnore
	FileExtensions     map[string][]string
	FilePushOrder      []string
	SkipSubdirectories bool
	// AllowSymlinks opts into following symlinked files during local file
	// collection. It is a CLI-level choice (--allow-symlinks /
	// GLASP_ALLOW_SYMLINKS), not part of .clasp.json, so callers set it after
	// building Options from config. When false (the default), symlinked files
	// are skipped rather than read, since a project shared over a symlinked
	// folder could otherwise cause push to read and upload arbitrary files
	// reachable via a link (e.g. one pointing outside the project).
	AllowSymlinks bool
	// OnSkip, when set, is called once for every candidate file
	// CollectLocalFiles decides not to collect for a reason a user might not
	// expect (matched .claspignore, a symlink, or a .d.ts declaration file).
	// It is not called for files that simply don't match a configured
	// extension (e.g. README.md in a typical project) — that is expected,
	// not surprising, behavior. Callers use this to surface a "N files
	// skipped" summary so a file missing from a push isn't a silent mystery.
	OnSkip func(SkippedFile)
}

// SkipReason identifies why CollectLocalFiles did not collect a candidate file.
type SkipReason string

const (
	// SkipReasonIgnored means the path matched .claspignore (or a built-in
	// default ignore pattern such as node_modules/).
	SkipReasonIgnored SkipReason = "ignored"
	// SkipReasonDeclaration means the file is a TypeScript .d.ts declaration
	// file, which is never deployable and is always excluded.
	SkipReasonDeclaration SkipReason = "declaration"
	// SkipReasonSymlink means the file is a symlink that was skipped because
	// --allow-symlinks was not set.
	SkipReasonSymlink SkipReason = "symlink"
)

// SkippedFile records one file CollectLocalFiles chose not to collect,
// reported via Options.OnSkip.
type SkippedFile struct {
	// Path is the file's path relative to ProjectRoot (slash-separated).
	Path   string
	Reason SkipReason
}

func reportSkip(onSkip func(SkippedFile), path string, reason SkipReason) {
	if onSkip == nil {
		return
	}
	onSkip(SkippedFile{Path: path, Reason: reason})
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
		FileTypeServerJS: {".js", ".gs", ".ts"},
		FileTypeHTML:     {".html"},
		FileTypeJSON:     {".json"},
	}
}

// CollectLocalFiles scans local files and returns push-ready project files.
func CollectLocalFiles(opts Options) ([]ProjectFile, error) {
	contentDir, err := contentDir(opts)
	if err != nil {
		return nil, err
	}
	// Resolved once for comparing against symlink targets (which
	// filepath.EvalSymlinks also fully resolves) further down, so the
	// comparison isn't thrown off by a symlink earlier in contentDir itself
	// (e.g. /tmp -> /private/tmp on macOS).
	resolvedContentDir, err := filepath.EvalSymlinks(contentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve rootDir: %w", err)
	}
	// .glasp/ holds internal data (tokens, archives, config) and must never
	// be collected, independent of rootDir/Ignore: a symlink whose target
	// resolves inside it (e.g. a tracked "Code.gs" pointing at
	// ".glasp/access.json") would otherwise pass the rootDir containment
	// check below and upload the token as if it were source code. Resolved
	// once here; ".glasp" need not exist yet (resolveExisting handles that).
	resolvedGlaspDir, err := resolveExisting(filepath.Join(opts.ProjectRoot, ".glasp"))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve .glasp directory: %w", err)
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
					Type:       FileTypeJSON,
					Source:     string(source),
				})
				conflicts["appsscript"] = ProjectFile{
					LocalPath: "appsscript.json",
					Type:      FileTypeJSON,
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

		// A symlink entry here is usually a symlinked file: for a symlink
		// found *inside* contentDir, filepath.WalkDir never descends into it
		// even when it points at a directory (its DirEntry reports
		// IsDir()==false), so a symlinked directory below contentDir is
		// simply never traversed, regardless of AllowSymlinks. The one
		// exception is currentPath itself being contentDir (WalkDir does
		// follow a symlinked root's *own* entry, though still without
		// descending into it) — a directory symlink can reach this branch,
		// which is why its type is resolved and checked before anything
		// else: a directory has no extension to match against
		// fileExtensions, so it must be handled here rather than falling
		// into the candidacy check below (which would otherwise silently
		// drop it, along with the only diagnostic that rootDir itself is a
		// symlink and nothing will be collected from it).
		isSymlink := entry.Type()&fs.ModeSymlink != 0
		var resolved string
		var targetIsDir, targetStatOK bool
		var resolveErr error
		if isSymlink {
			resolved, resolveErr = filepath.EvalSymlinks(currentPath)
			if resolveErr == nil {
				if info, statErr := os.Stat(resolved); statErr == nil {
					targetIsDir = info.IsDir()
					targetStatOK = true
				}
			}
		}
		if isSymlink && targetIsDir {
			// Directory symlinks are never followed, with or without
			// --allow-symlinks (WalkDir doesn't descend into one, and this
			// is the one case — the walk's own root — where that matters).
			slog.Debug("skipping symlinked directory (directory symlinks are never followed, even with --allow-symlinks)", "path", relToRoot)
			reportSkip(opts.OnSkip, relToRoot, SkipReasonSymlink)
			return nil
		}

		// Determine candidacy — would this path ever be collected — before
		// consulting .claspignore or validating a symlinked *file*. A path
		// that was never going to be collected anyway (wrong extension, and
		// not a .d.ts) must not be reported via OnSkip: otherwise "Skipped N
		// file(s)" would count every non-matching file under an ignored
		// directory (e.g. every file inside node_modules/), not just the
		// ones a user might actually expect to see pushed. It also must not
		// be symlink-validated: since it is never read, an unresolvable or
		// escaping link target here poses no risk.
		isDeclaration := strings.HasSuffix(strings.ToLower(relToRoot), ".d.ts")
		fileType := fileTypeForPath(relToRoot, fileExtensions)
		if !isDeclaration && fileType == "" {
			return nil
		}

		if opts.Ignore != nil && opts.Ignore.Matches(relToRoot) {
			reportSkip(opts.OnSkip, relToRoot, SkipReasonIgnored)
			return nil
		}
		// Symlinked files are skipped by default: a project shared over a
		// symlinked folder could otherwise let push silently read (and
		// upload) a file reachable via a link pointing outside the project,
		// e.g. a link to a secrets file or another user's home directory.
		// --allow-symlinks / GLASP_ALLOW_SYMLINKS opts back in, but even then
		// the link target must resolve inside contentDir and outside
		// .glasp/ (a tracked file symlinked to .glasp/access.json would
		// otherwise pass the rootDir containment check and upload the auth
		// token as if it were source).
		if isSymlink {
			// Per-file detail is logged at debug level only — the one-line
			// "Skipped N file(s)" summary is what's shown by default (see
			// OnSkip/skipTracker in cmd/glasp).
			if !opts.AllowSymlinks {
				slog.Debug("skipping symlinked file (use --allow-symlinks to include it)", "path", relToRoot)
				reportSkip(opts.OnSkip, relToRoot, SkipReasonSymlink)
				return nil
			}
			if resolveErr != nil {
				return fmt.Errorf("failed to resolve symlink %s: %w", relToRoot, resolveErr)
			}
			if !targetStatOK {
				return fmt.Errorf("failed to stat symlink target for %s", relToRoot)
			}
			within, err := isWithin(resolvedContentDir, resolved)
			if err != nil {
				return fmt.Errorf("invalid symlink target for %s: %w", relToRoot, err)
			}
			if !within {
				return fmt.Errorf("symlink %s resolves outside rootDir; refusing to follow it even with --allow-symlinks", relToRoot)
			}
			insideGlasp, err := isWithin(resolvedGlaspDir, resolved)
			if err != nil {
				return fmt.Errorf("invalid symlink target for %s: %w", relToRoot, err)
			}
			if insideGlasp {
				return fmt.Errorf("symlink %s resolves inside .glasp/, which is never collected", relToRoot)
			}
		}
		if isDeclaration {
			// TypeScript declaration files (.d.ts) are never deployable.
			reportSkip(opts.OnSkip, relToRoot, SkipReasonDeclaration)
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
		// For a symlink, read from the already-resolved, symlink-free path
		// rather than re-opening currentPath (the link) again: re-opening
		// the link here would race a concurrent retarget of the symlink
		// between the validation above and this read (check-then-use), so
		// what gets uploaded may not be what was actually validated. Reading
		// the resolved path instead means a retarget after validation has
		// no effect — the content read is the one that was checked.
		readPath := currentPath
		if isSymlink {
			readPath = resolved
		}
		source, err := os.ReadFile(readPath)
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
		if err := fsutil.WriteFileAtomic(targetPath, []byte(file.Source), 0644); err != nil {
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
	// Resolved once, using the deepest *existing* ancestor when contentDir
	// itself doesn't exist yet (create/clone write into a rootDir before
	// it's been created). Used below to catch a pre-existing symlink in
	// targetPath's parent chain — e.g. a "pkg" directory that is actually a
	// symlink to somewhere outside the project, left behind by a cloned
	// repository or a previous compromised state — before writing through
	// it. The lexical checks further down only validate the remote name's
	// spelling; they don't see what's already on disk.
	resolvedContentDir, err := resolveExisting(contentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve rootDir: %w", err)
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
		if !validateRelPath(file.Name) {
			return nil, fmt.Errorf("invalid remote file name: %s", file.Name)
		}
		remoteName := path.Clean(filepath.ToSlash(file.Name))
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
		// Resolve the deepest existing ancestor of the parent directory
		// *before* creating anything: if it turns out to already be (or sit
		// inside) a symlink pointing outside the project, refuse to write
		// rather than silently creating/following through it. Once this
		// check passes, MkdirAll only ever creates new, plain (non-symlink)
		// directories for the remaining path, so nothing created here can
		// itself introduce an escape.
		resolvedParent, err := resolveExisting(filepath.Dir(targetPath))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve target directory for %s: %w", file.Name, err)
		}
		within, err := isWithin(resolvedContentDir, resolvedParent)
		if err != nil {
			return nil, fmt.Errorf("invalid remote file name: %s: %w", file.Name, err)
		}
		if !within {
			return nil, fmt.Errorf("remote file %q would be written outside rootDir via an existing symlink", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, err
		}
		source := file.Source
		// Written atomically (temp file + rename) so a pull interrupted
		// mid-write (API error, signal, crash) never leaves a local file
		// half-written; the previous content or nothing is observed, never
		// a corrupt partial file.
		if err := fsutil.WriteFileAtomic(targetPath, []byte(source), 0644); err != nil {
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

// validateRelPath reports whether raw is usable as a safe relative path:
// non-empty, no NUL bytes, no UNC prefix, not absolute, no Windows drive
// prefix, and no ".." elements. Callers remain responsible for cleaning the
// path and verifying the joined result stays inside their base directory.
func validateRelPath(raw string) bool {
	if raw == "" {
		return false
	}
	if strings.Contains(raw, "\x00") {
		return false
	}
	if strings.HasPrefix(raw, `\\`) {
		return false
	}
	slashed := filepath.ToSlash(raw)
	if strings.HasPrefix(slashed, "/") {
		return false
	}
	if len(slashed) >= 2 && slashed[1] == ':' {
		drive := slashed[0]
		if (drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z') {
			return false
		}
	}
	for _, part := range strings.Split(slashed, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func contentDir(opts Options) (string, error) {
	if opts.ProjectRoot == "" {
		return "", fmt.Errorf("project root is empty")
	}
	rootDir := strings.TrimSpace(opts.RootDir)
	if rootDir == "" {
		rootDir = "."
	}
	dir := filepath.Join(opts.ProjectRoot, filepath.Clean(rootDir))
	// Reject a rootDir/srcDir (from .clasp.json, which may come from a
	// cloned/untrusted repository) that resolves outside the project root.
	// This mirrors the traversal guard already applied to remote file names
	// in ApplyRemoteContent.
	//
	// The check is done on symlink-resolved paths, not just lexically: a
	// lexically-inside rootDir such as "link/src" escapes the project when
	// "link" is a symlink to an outside directory (a symlink a cloned
	// repository can carry). A lexical filepath.Rel would accept that and
	// let push read — and pull write — files outside the project.
	resolvedRoot, err := resolveExisting(opts.ProjectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root: %w", err)
	}
	resolvedDir, err := resolveExisting(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve rootDir %q: %w", opts.RootDir, err)
	}
	within, err := isWithin(resolvedRoot, resolvedDir)
	if err != nil {
		return "", fmt.Errorf("invalid rootDir %q: %w", opts.RootDir, err)
	}
	if !within {
		return "", fmt.Errorf("rootDir %q resolves outside the project root", opts.RootDir)
	}
	return dir, nil
}

// isWithin reports whether target is base itself or lies inside it. Callers
// pass already symlink-resolved, absolute paths (e.g. via resolveExisting)
// so this is a pure lexical containment check on paths that reflect the real
// filesystem location, not just how they were spelled.
func isWithin(base, target string) (bool, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false, err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return false, nil
	}
	return true, nil
}

// resolveExisting resolves symlinks in path. A path that does not exist yet
// (create/clone write into a rootDir before it exists) cannot be resolved
// directly, so the deepest existing ancestor is resolved instead and the
// remaining, not-yet-created components are appended to it. Those components
// are plain names — they cannot be symlinks, because they do not exist — so
// the result is still a faithful resolution for containment checks.
func resolveExisting(path string) (string, error) {
	remaining := ""
	current := filepath.Clean(path)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if remaining == "" {
				return resolved, nil
			}
			return filepath.Join(resolved, remaining), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached the filesystem root without finding an existing
			// ancestor: nothing to resolve, use the path as given.
			return filepath.Clean(path), nil
		}
		remaining = filepath.Join(filepath.Base(current), remaining)
		current = parent
	}
}

func fileTypeForPath(localPath string, fileExtensions map[string][]string) string {
	ext := strings.ToLower(path.Ext(localPath))
	if matchesExtension(ext, fileExtensions[FileTypeServerJS]) {
		return FileTypeServerJS
	}
	if matchesExtension(ext, fileExtensions[FileTypeHTML]) {
		return FileTypeHTML
	}
	if matchesExtension(ext, fileExtensions[FileTypeJSON]) && path.Base(localPath) == "appsscript.json" {
		return FileTypeJSON
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
	if fileType == FileTypeJSON && path.Base(relToContent) == "appsscript.json" {
		remotePath = "appsscript"
	}
	return remotePath
}

func extensionForType(fileType string, fileExtensions map[string][]string) (string, error) {
	switch fileType {
	case FileTypeServerJS:
		return firstExtension(fileExtensions[FileTypeServerJS]), nil
	case FileTypeHTML:
		return firstExtension(fileExtensions[FileTypeHTML]), nil
	case FileTypeJSON:
		return firstExtension(fileExtensions[FileTypeJSON]), nil
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
		FileTypeServerJS: normalizeExtensions(scriptExtensions),
		FileTypeHTML:     normalizeExtensions(htmlExtensions),
		FileTypeJSON:     normalizeExtensions(jsonExtensions),
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
	if !validateRelPath(localPath) {
		return "", fmt.Errorf("invalid local path: %s", localPath)
	}
	cleaned := filepath.Clean(filepath.FromSlash(filepath.ToSlash(localPath)))
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
