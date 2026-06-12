package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/syncer"
	"github.com/takihito/glasp/internal/transform"

	"github.com/alecthomas/kong"
	"google.golang.org/api/script/v1"
)

const fileTypeServerJSForConvert = "SERVER_JS"

// ConvertCmd represents the 'convert' subcommand.
type ConvertCmd struct {
	GasToTS bool     `help:"Convert GAS JavaScript (.js/.gs) to TypeScript."`
	TSToGas bool     `help:"Convert TypeScript (.ts) to GAS JavaScript."`
	Targets []string `arg:"" optional:"" name:"path" help:"Target files or directories to convert."`
}

// Run executes the convert command.
func (c *ConvertCmd) Run(ctx *kong.Context) error {
	mode, err := transformMode(c.GasToTS, c.TSToGas)
	if err != nil {
		return err
	}
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	if projectRoot == "" {
		return fmt.Errorf(".clasp.json not found in current or parent directories")
	}
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return err
	}
	ignore, err := config.NewClaspIgnore(projectRoot)
	if err != nil {
		return err
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, ignore)
	if err != nil {
		return err
	}
	opts.FileExtensions = transformFileExtensions(opts.FileExtensions, mode)
	outDir := defaultTransformOutDir(projectRoot, mode)
	filter, err := transform.NewTargetFilter(projectRoot, c.Targets)
	if err != nil {
		return err
	}
	result, err := transformConvertFn(opts, outDir, mode, filter)
	if err != nil {
		return err
	}
	fmt.Printf("Converted %d files to %s\n", len(result.Written), result.OutDir)
	return nil
}

func transformMode(gasToTS, tsToGas bool) (transform.Mode, error) {
	if gasToTS && tsToGas {
		return "", fmt.Errorf("choose either --gas-to-ts or --ts-to-gas")
	}
	if gasToTS {
		return transform.ModeGasToTS, nil
	}
	if tsToGas {
		return transform.ModeTSToGas, nil
	}
	return "", fmt.Errorf("either --gas-to-ts or --ts-to-gas is required")
}

func transformFileExtensions(existing map[string][]string, mode transform.Mode) map[string][]string {
	if existing == nil {
		existing = syncer.DefaultFileExtensions()
	}
	switch mode {
	case transform.ModeGasToTS:
		existing["SERVER_JS"] = []string{".js", ".gs"}
	case transform.ModeTSToGas:
		existing["SERVER_JS"] = []string{".ts"}
	}
	return existing
}

func defaultTransformOutDir(projectRoot string, mode transform.Mode) string {
	switch mode {
	case transform.ModeTSToGas:
		return filepath.Join(projectRoot, ".glasp", "dist", "gas")
	case transform.ModeGasToTS:
		fallthrough
	default:
		return filepath.Join(projectRoot, ".glasp", "dist", "ts")
	}
}

func convertPulledContent(content *script.Content, projectRoot string) (*script.Content, error) {
	if content == nil {
		return nil, fmt.Errorf("content is nil")
	}
	out := &script.Content{
		ScriptId: content.ScriptId,
		Files:    make([]*script.File, 0, len(content.Files)),
	}
	for _, file := range content.Files {
		if file == nil {
			continue
		}
		cloned := &script.File{
			Name:   file.Name,
			Type:   file.Type,
			Source: file.Source,
		}
		if file.Type == fileTypeServerJSForConvert {
			converted, err := transform.ConvertServerJSSource(transform.ModeGasToTS, file.Source, file.Name+".js", projectRoot)
			if err != nil {
				return nil, err
			}
			cloned.Source = converted
		}
		out.Files = append(out.Files, cloned)
	}
	return out, nil
}

func convertPushFiles(files []syncer.ProjectFile, projectRoot string) ([]syncer.ProjectFile, error) {
	converted := make([]syncer.ProjectFile, 0, len(files))
	for _, file := range files {
		if file.Type == fileTypeServerJSForConvert && isTypeScriptPath(file.LocalPath) {
			lowerPath := strings.ToLower(file.LocalPath)
			if strings.HasSuffix(lowerPath, ".d.ts") {
				continue
			}
			out, err := transform.ConvertServerJSSource(transform.ModeTSToGas, file.Source, file.LocalPath, projectRoot)
			if err != nil {
				return nil, err
			}
			file.Source = out
			file.LocalPath = payloadJSPath(file.LocalPath)
		}
		converted = append(converted, file)
	}
	return converted, nil
}

// isTypeScriptPath returns true if the file path has a .ts extension,
// excluding .d.ts declaration files and .tsx (unsupported for auto-conversion).
func isTypeScriptPath(localPath string) bool {
	lower := strings.ToLower(localPath)
	return strings.HasSuffix(lower, ".ts") && !strings.HasSuffix(lower, ".d.ts")
}

// hasTypeScriptFiles returns true if any collected file is a TypeScript source.
func hasTypeScriptFiles(files []syncer.ProjectFile) bool {
	for _, f := range files {
		if isTypeScriptPath(f.LocalPath) {
			return true
		}
	}
	return false
}

func payloadJSPath(localPath string) string {
	ext := filepath.Ext(localPath)
	if ext == "" {
		return localPath
	}
	return strings.TrimSuffix(localPath, ext) + ".js"
}

// normalizeFileExtension lowercases a file extension and strips surrounding
// whitespace and a leading dot ("  .TS " -> "ts").
func normalizeFileExtension(raw string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), ".")
}

func claspFileExtension(cfg *config.ClaspConfig) string {
	if cfg == nil || cfg.Extra == nil {
		return ""
	}
	raw, ok := cfg.Extra["fileExtension"]
	if !ok {
		return ""
	}
	var fileExtension string
	if err := json.Unmarshal(raw, &fileExtension); err != nil {
		return ""
	}
	return normalizeFileExtension(fileExtension)
}

func isTypeScriptFileExtension(fileExtension string) bool {
	switch normalizeFileExtension(fileExtension) {
	case "ts", "tsx":
		return true
	default:
		return false
	}
}

func isTSXFileExtension(fileExtension string) bool {
	return normalizeFileExtension(fileExtension) == "tsx"
}

func validateSupportedSyncFileExtension(fileExtension string) error {
	if isTSXFileExtension(fileExtension) {
		return fmt.Errorf("fileExtension \"tsx\" is not supported for pull/push auto conversion; use \"ts\" or run convert manually")
	}
	return nil
}

func dryRunConvertLabelForPull(fileExtension string) string {
	if isTypeScriptFileExtension(fileExtension) {
		return transform.ModeGasToTS.Label()
	}
	return transform.Mode("").Label()
}
