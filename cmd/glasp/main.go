package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/takihito/glasp/internal/browser"
	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/history"
	"github.com/takihito/glasp/internal/transform"

	"github.com/alecthomas/kong"
)

var (
	newScriptClientWithCacheAuthFn           = newScriptClientWithCachePathAndAuth
	transformConvertFn                       = transform.Convert
	convertPulledContentFn                   = convertPulledContent
	openURLFn                                = browser.Open
	marshalJSONFn                            = json.Marshal
	stdout                         io.Writer = os.Stdout
	stderr                         io.Writer = os.Stderr
)

type runArchiveMeta struct {
	Enabled   bool
	Direction string
	Path      string
}

// defaultHTTPTimeout is applied to every Script API HTTP request when no
// explicit --timeout flag or .glasp/config.json value is provided.
const defaultHTTPTimeout = 180 * time.Second

// runContext carries per-invocation state into command Run methods via kong
// bindings: the base context and the archive metadata recorded into history.
// All methods tolerate a nil receiver so tests can invoke Run(nil).
type runContext struct {
	ctx         context.Context
	archive     runArchiveMeta
	httpTimeout time.Duration
}

// Context returns the invocation context.
func (rc *runContext) Context() context.Context {
	if rc == nil || rc.ctx == nil {
		return context.Background()
	}
	return rc.ctx
}

// HTTPTimeout returns the resolved HTTP timeout for API requests.
func (rc *runContext) HTTPTimeout() time.Duration {
	if rc == nil {
		return 0
	}
	return rc.httpTimeout
}

func (rc *runContext) setArchiveMeta(enabled bool, direction string) {
	if rc == nil {
		return
	}
	rc.archive = runArchiveMeta{Enabled: enabled, Direction: direction}
}

func (rc *runContext) setArchivePath(path string) {
	if rc == nil {
		return
	}
	rc.archive.Path = path
}

func (rc *runContext) archiveMeta() runArchiveMeta {
	if rc == nil {
		return runArchiveMeta{}
	}
	return rc.archive
}

// CLI is the main command-line interface structure for glasp.
type CLI struct {
	Dir              string              `name:"dir" short:"C" env:"GLASP_DIR" help:"Change to this directory before executing any command."`
	Timeout          int                 `name:"timeout" env:"GLASP_TIMEOUT" help:"HTTP timeout for Script API requests in seconds. 0 = use .glasp/config.json value or default (180s)."`
	NoTimeout        bool                `name:"no-timeout" env:"GLASP_NO_TIMEOUT" help:"Disable HTTP timeout for Script API requests (unlimited). Overrides --timeout and .glasp/config.json."`
	Login            LoginCmd            `cmd:"" help:"Log in to Google account."`
	Logout           LogoutCmd           `cmd:"" help:"Log out from Google account."`
	CreateScript     CreateCmd           `cmd:"" name:"create-script" aliases:"create" help:"Create a new Apps Script project."`
	Clone            CloneCmd            `cmd:"" help:"Clone an existing Apps Script project."`
	Pull             PullCmd             `cmd:"" help:"Download project files from Apps Script."`
	Push             PushCmd             `cmd:"" help:"Upload project files to Apps Script."`
	OpenScript       OpenScriptCmd       `cmd:"" name:"open-script" aliases:"open" help:"Open the Apps Script project in browser."`
	CreateDeployment CreateDeploymentCmd `cmd:"" name:"create-deployment" help:"Create a deployment (or redeploy with --deploymentId)."`
	UpdateDeployment UpdateDeploymentCmd `cmd:"" name:"update-deployment" aliases:"deploy" help:"Update an existing deployment."`
	ListDeployments  ListDeploymentsCmd  `cmd:"" name:"list-deployments" help:"List deployments for a script project."`
	RunFunction      RunFunctionCmd      `cmd:"" name:"run-function" help:"Run an Apps Script function remotely."`
	Convert          ConvertCmd          `cmd:"" help:"Convert project files with esbuild."`
	History          HistoryCmd          `cmd:"" help:"Show command execution history."`
	Config           ConfigCmd           `cmd:"" help:"Manage glasp config."`
	Version          VersionCmd          `cmd:"" help:"Show glasp version."`
}

func main() {
	start := time.Now()
	rawArgs := append([]string(nil), os.Args[1:]...)
	commandName := commandFromArgs(rawArgs)

	var cli CLI
	parsed := kong.Parse(&cli,
		kong.Name("glasp"),
		kong.Description("A Go-based Google Apps Script CLI (clasp alternative)."),
		kong.UsageOnError(),
	)
	if dir := strings.TrimSpace(cli.Dir); dir != "" {
		if err := os.Chdir(dir); err != nil {
			log.Fatalf("Error: failed to change directory to %q: %v", dir, err)
		}
	}
	rc := &runContext{
		ctx:         context.Background(),
		httpTimeout: resolveHTTPTimeout(cli.Timeout, cli.NoTimeout),
	}
	err := parsed.Run(rc)
	recordRunHistory(rawArgs, commandName, time.Since(start), err, rc.archiveMeta())
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

// resolveHTTPTimeout returns the HTTP timeout to use for Script API requests.
// Priority: --no-timeout > --timeout / GLASP_TIMEOUT env > .glasp/config.json > defaultHTTPTimeout.
// Returns 0 when noTimeout is true, which net/http interprets as no timeout.
func resolveHTTPTimeout(flagSeconds int, noTimeout bool) time.Duration {
	if noTimeout {
		return 0
	}
	if flagSeconds > 0 {
		return time.Duration(flagSeconds) * time.Second
	}
	projectRoot, err := config.FindProjectRoot()
	if err == nil && projectRoot != "" {
		glaspCfg, err := config.LoadGlaspConfig(projectRoot)
		if err != nil {
			// LoadGlaspConfig returns an error only when the file exists but is
			// malformed/unreadable (a missing file yields a zero-value config).
			// Warn instead of silently honoring the default so a timeoutSeconds
			// typo is visible to the user rather than ignored.
			log.Printf("Warning: failed to load .glasp/config.json; using default timeout (%s): %v", defaultHTTPTimeout, err)
		} else if glaspCfg.TimeoutSeconds > 0 {
			return time.Duration(glaspCfg.TimeoutSeconds) * time.Second
		}
	}
	return defaultHTTPTimeout
}

// valuedGlobalFlags lists root-level flags that take a separate value token
// (e.g. `--timeout 60`). Because these flags may be placed before the
// subcommand, commandFromArgs must skip their value so it is not mistaken for
// the command name. Boolean flags such as --no-timeout are intentionally
// absent. The `--flag=value` form carries its own value and needs no entry.
var valuedGlobalFlags = map[string]bool{
	"--dir":     true,
	"-C":        true,
	"--timeout": true,
}

func commandFromArgs(args []string) string {
	// Collect the first two positional tokens, skipping flags and the value
	// tokens of valued global flags placed before the subcommand.
	positionals := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if valuedGlobalFlags[arg] && i+1 < len(args) {
				i++ // skip this flag's value token
			}
			continue
		}
		positionals = append(positionals, arg)
		if len(positionals) == 2 {
			break
		}
	}
	if len(positionals) == 0 {
		return ""
	}

	// Keep aliases as entered, but include known nested subcommands
	// so `config init` is distinguishable from the command group itself.
	if positionals[0] == "config" && len(positionals) > 1 {
		return positionals[0] + " " + positionals[1]
	}
	return positionals[0]
}

func recordRunHistory(args []string, commandName string, duration time.Duration, runErr error, archiveMeta runArchiveMeta) {
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		log.Printf("Warning: failed to resolve project root for history: %v", err)
		return
	}
	if projectRoot == "" {
		return
	}
	status := "success"
	message := ""
	if runErr != nil {
		status = "error"
		message = runErr.Error()
	}
	entry := history.Entry{
		Timestamp:  time.Now().Format(time.RFC3339),
		Command:    commandName,
		Args:       sanitizeHistoryArgs(args),
		Status:     status,
		Error:      message,
		DurationMs: duration.Milliseconds(),
		Archive: history.Archive{
			Enabled:   archiveMeta.Enabled,
			Direction: archiveMeta.Direction,
			Path:      archiveMeta.Path,
		},
	}
	if err := history.Append(projectRoot, entry); err != nil {
		log.Printf("Warning: failed to append history entry: %v", err)
	}
}

// sensitiveShortFlags maps short flags to their redaction status.
// Short flags whose long-form names contain sensitive keywords must be
// listed here because isSensitiveOption only inspects "--" prefixed names.
var sensitiveShortFlags = map[string]bool{
	"-p": true, // --params
}

func sanitizeHistoryArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	sensitiveKeywords := []string{
		"auth",
		"token",
		"api-key",
		"apikey",
		"password",
		"secret",
		"params",
		"key",
	}
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if eq := strings.Index(arg, "="); strings.HasPrefix(arg, "--") && eq > 0 {
			key := arg[:eq]
			if isSensitiveOption(key, sensitiveKeywords) {
				out = append(out, key+"=REDACTED")
				continue
			}
		}
		if isSensitiveOption(arg, sensitiveKeywords) || sensitiveShortFlags[arg] {
			out = append(out, arg)
			if i+1 < len(args) {
				out = append(out, "REDACTED")
				i++
			}
			continue
		}
		out = append(out, arg)
	}
	return out
}

func isSensitiveOption(opt string, keywords []string) bool {
	if !strings.HasPrefix(opt, "--") {
		return false
	}
	name := strings.TrimPrefix(opt, "--")
	lower := strings.ToLower(name)
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
