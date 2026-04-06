package transform

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/takihito/glasp/internal/syncer"

	"github.com/evanw/esbuild/pkg/api"
)

// Mode defines the conversion direction.
type Mode string

const (
	ModeGasToTS Mode = "gas-to-ts"
	ModeTSToGas Mode = "ts-to-gas"
)

const (
	fileTypeServerJS = "SERVER_JS"
)

var importExportPattern = regexp.MustCompile(`\b(import|export)\b`)

// Result summarizes the conversion output.
type Result struct {
	OutDir  string
	Written []string
}

// TargetFilter limits which project-relative paths should be converted.
type TargetFilter struct {
	files map[string]struct{}
	dirs  []string
}

// Convert transforms local project files into the specified output directory.
func Convert(opts syncer.Options, outDir string, mode Mode, filter *TargetFilter) (Result, error) {
	if outDir == "" {
		return Result{}, fmt.Errorf("output directory is required")
	}
	if mode != ModeGasToTS && mode != ModeTSToGas {
		return Result{}, fmt.Errorf("unsupported transform mode: %s", mode)
	}
	files, err := syncer.CollectLocalFiles(opts)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return Result{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	var written []string
	for _, file := range files {
		if !filterAllows(filter, file.LocalPath) {
			continue
		}
		if mode == ModeTSToGas && strings.HasSuffix(strings.ToLower(file.LocalPath), ".d.ts") {
			continue
		}
		targetPath, err := outputPath(file, opts.RootDir, outDir, mode)
		if err != nil {
			return Result{}, err
		}
		if err := ensureWithinOutDir(outDir, targetPath); err != nil {
			return Result{}, err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return Result{}, fmt.Errorf("failed to create output directory: %w", err)
		}
		source := file.Source
		if file.Type == fileTypeServerJS {
			if mode == ModeTSToGas {
				warnImportExport(file.LocalPath, source)
				resolveDir := resolveDirForFile(opts.ProjectRoot, file.LocalPath)
				converted, err := runEsbuildTSToGas(source, file.LocalPath, resolveDir)
				if err != nil {
					return Result{}, err
				}
				source = converted
			} else {
				converted, err := runEsbuildGasToTS(source, file.LocalPath)
				if err != nil {
					return Result{}, err
				}
				source = converted
			}
		}
		if err := os.WriteFile(targetPath, []byte(source), 0644); err != nil {
			return Result{}, fmt.Errorf("failed to write %s: %w", targetPath, err)
		}
		logConverted(file.LocalPath, outDir, targetPath)
		written = append(written, targetPath)
	}
	return Result{OutDir: outDir, Written: written}, nil
}

func outputPath(file syncer.ProjectFile, rootDir, outDir string, mode Mode) (string, error) {
	if file.LocalPath == "" {
		return "", fmt.Errorf("local path is empty")
	}
	if file.LocalPath == "appsscript.json" {
		return filepath.Join(outDir, "appsscript.json"), nil
	}
	rel := filepath.ToSlash(file.LocalPath)
	rootPrefix := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(rootDir)), "/")
	if rootPrefix != "" && rootPrefix != "." {
		prefix := rootPrefix + "/"
		if strings.HasPrefix(rel, prefix) {
			rel = strings.TrimPrefix(rel, prefix)
		}
	}
	if file.Type == fileTypeServerJS {
		base := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
		dir := path.Dir(rel)
		ext := ".ts"
		if mode == ModeTSToGas {
			ext = ".js"
		}
		if dir == "." {
			rel = base + ext
		} else {
			rel = path.Join(dir, base+ext)
		}
	}
	return filepath.Join(outDir, filepath.FromSlash(rel)), nil
}

func ensureWithinOutDir(outDir, targetPath string) error {
	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve output path: %w", err)
	}
	rel, err := filepath.Rel(outAbs, targetAbs)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return fmt.Errorf("output path escapes output directory: %s", targetPath)
	}
	return nil
}

func NewTargetFilter(projectRoot string, targets []string) (*TargetFilter, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project root: %w", err)
	}
	filter := &TargetFilter{
		files: make(map[string]struct{}),
	}
	for _, raw := range targets {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		targetPath := raw
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(root, targetPath)
		}
		absTarget, err := filepath.Abs(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve target %s: %w", raw, err)
		}
		rel, err := filepath.Rel(root, absTarget)
		if err != nil {
			return nil, fmt.Errorf("invalid target %s: %w", raw, err)
		}
		rel = filepath.ToSlash(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, fmt.Errorf("target path escapes project root: %s", raw)
		}
		if rel == "." {
			return nil, nil
		}
		info, err := os.Stat(absTarget)
		if err != nil {
			return nil, fmt.Errorf("failed to stat target %s: %w", raw, err)
		}
		if info.IsDir() {
			prefix := strings.TrimSuffix(rel, "/") + "/"
			filter.dirs = append(filter.dirs, prefix)
			continue
		}
		filter.files[rel] = struct{}{}
	}
	if len(filter.files) == 0 && len(filter.dirs) == 0 {
		return nil, nil
	}
	return filter, nil
}

func (tf *TargetFilter) Empty() bool {
	if tf == nil {
		return true
	}
	return len(tf.files) == 0 && len(tf.dirs) == 0
}

func filterAllows(filter *TargetFilter, localPath string) bool {
	if filter == nil {
		return true
	}
	if localPath == "" {
		return false
	}
	localPath = filepath.ToSlash(localPath)
	if _, ok := filter.files[localPath]; ok {
		return true
	}
	for _, prefix := range filter.dirs {
		if strings.HasPrefix(localPath, prefix) {
			return true
		}
	}
	return false
}

func logConverted(srcPath, outDir, targetPath string) {
	if strings.TrimSpace(srcPath) == "" || strings.TrimSpace(targetPath) == "" {
		return
	}
	relTarget, err := filepath.Rel(outDir, targetPath)
	if err != nil {
		relTarget = targetPath
	}
	relTarget = filepath.ToSlash(relTarget)
	timestamp := time.Now().Format(time.RFC3339)
	log.Printf("Converted %s -> %s at %s", srcPath, relTarget, timestamp)
}

func warnImportExport(localPath, source string) {
	if strings.TrimSpace(localPath) == "" || source == "" {
		return
	}
	lines := strings.Split(source, "\n")
	inBlock := false
	for idx, line := range lines {
		if line == "" && !inBlock {
			continue
		}
		scan := line
		if inBlock {
			if end := strings.Index(scan, "*/"); end >= 0 {
				scan = scan[end+2:]
				inBlock = false
			} else {
				continue
			}
		}
		if start := strings.Index(scan, "/*"); start >= 0 {
			if end := strings.Index(scan[start+2:], "*/"); end >= 0 {
				scan = scan[:start] + scan[start+2+end+2:]
			} else {
				scan = scan[:start]
				inBlock = true
			}
		}
		if slash := strings.Index(scan, "//"); slash >= 0 {
			scan = scan[:slash]
		}
		if !importExportPattern.MatchString(scan) {
			continue
		}
		lineNo := idx + 1
		log.Printf("Warning: import/export detected in %s:%d", localPath, lineNo)
	}
}

// ConvertServerJSSource converts a single SERVER_JS source in memory.
func ConvertServerJSSource(mode Mode, source, sourcefile, projectRoot string) (string, error) {
	switch mode {
	case ModeGasToTS:
		return runEsbuildGasToTS(source, sourcefile)
	case ModeTSToGas:
		warnImportExport(sourcefile, source)
		resolveDir := resolveDirForFile(projectRoot, sourcefile)
		return runEsbuildTSToGas(source, sourcefile, resolveDir)
	default:
		return "", fmt.Errorf("unsupported transform mode: %s", mode)
	}
}

func runEsbuildGasToTS(source, sourcefile string) (string, error) {
	options := baseBuildOptions()
	options.Stdin = &api.StdinOptions{
		Contents:   source,
		Sourcefile: sourcefile,
		Loader:     api.LoaderJS,
	}
	options.Bundle = false
	return runEsbuild(options)
}

func runEsbuildTSToGas(source, sourcefile, resolveDir string) (string, error) {
	options := baseBuildOptions()
	options.Stdin = &api.StdinOptions{
		Contents:   source,
		Sourcefile: sourcefile,
		Loader:     api.LoaderTS,
		ResolveDir: resolveDir,
	}
	options.Bundle = false
	return runEsbuild(options)
}

func runEsbuild(options api.BuildOptions) (string, error) {
	result := api.Build(options)
	if len(result.Errors) > 0 {
		return "", fmt.Errorf("esbuild failed: %s", formatMessages(result.Errors))
	}
	if len(result.OutputFiles) == 0 {
		return "", fmt.Errorf("esbuild returned no output")
	}
	return string(result.OutputFiles[0].Contents), nil
}

func baseBuildOptions() api.BuildOptions {
	return api.BuildOptions{
		Write:       false,
		Format:      api.FormatESModule,
		Platform:    api.PlatformNeutral,
		Target:      api.ES2019,
		TreeShaking: api.TreeShakingFalse,
		LogLevel:    api.LogLevelSilent,
		Outfile:     "out.js",
	}
}

func resolveDirForFile(projectRoot, localPath string) string {
	if projectRoot == "" || localPath == "" {
		return ""
	}
	dir := filepath.Dir(filepath.FromSlash(localPath))
	abs := filepath.Join(projectRoot, dir)
	resolved, err := filepath.Abs(abs)
	if err != nil {
		return ""
	}
	return resolved
}

func formatMessages(messages []api.Message) string {
	formatted := api.FormatMessages(messages, api.FormatMessagesOptions{
		Kind:  api.ErrorMessage,
		Color: false,
	})
	return strings.Join(formatted, "\n")
}
