package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// defaultHTTPRetries is the number of retry attempts for transient Script API
// failures when no explicit --max-retries flag or .glasp/config.json value is
// provided. Only idempotent commands use retries (see retryableCommands).
const defaultHTTPRetries = 3

// runContext carries per-invocation state into command Run methods via kong
// bindings: the base context and the archive metadata recorded into history.
// All methods tolerate a nil receiver so tests can invoke Run(nil).
type runContext struct {
	ctx         context.Context
	archive     runArchiveMeta
	httpTimeout time.Duration
	httpRetries int
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

// HTTPRetries returns the resolved retry count for Script API requests.
// Returns 0 when rc is nil (no retries).
func (rc *runContext) HTTPRetries() int {
	if rc == nil {
		return 0
	}
	return rc.httpRetries
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
	LogLevel         string              `name:"log-level" env:"GLASP_LOG_LEVEL" enum:"debug,info,warn,error" default:"info" help:"Minimum level for diagnostic logs on stderr (debug|info|warn|error)."`
	LogFormat        string              `name:"log-format" env:"GLASP_LOG_FORMAT" enum:"text,json" default:"text" help:"Diagnostic log format (text|json)."`
	Timeout          int                 `name:"timeout" env:"GLASP_TIMEOUT" help:"HTTP timeout for Script API requests in seconds. 0 = use .glasp/config.json value or default (180s)."`
	NoTimeout        bool                `name:"no-timeout" env:"GLASP_NO_TIMEOUT" help:"Disable HTTP timeout for Script API requests (unlimited). Overrides --timeout and .glasp/config.json."`
	MaxRetries       int                 `name:"max-retries" env:"GLASP_MAX_RETRIES" help:"Max retry attempts for transient Script API failures (5xx/429/network). Applies only to idempotent commands (push, pull, list-deployments, clone). 0 = use .glasp/config.json value or default (3)."`
	NoRetries        bool                `name:"no-retries" env:"GLASP_NO_RETRIES" help:"Disable retries for Script API requests. Overrides --max-retries and .glasp/config.json."`
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

	var cli CLI
	parsed := kong.Parse(&cli,
		kong.Name("glasp"),
		kong.Description("A Go-based Google Apps Script CLI (clasp alternative)."),
		kong.UsageOnError(),
	)
	// Derive the history command name from kong's parsed selection rather than
	// re-parsing os.Args ourselves. kong.Parse exits the process on a parse
	// error, so the selection is always valid here.
	commandName := selectedCommandName(parsed)
	slog.SetDefault(slog.New(newLogHandler(stderr, cli.LogLevel, cli.LogFormat)))
	if dir := strings.TrimSpace(cli.Dir); dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(stderr, "Error: failed to change directory to %q: %v\n", dir, err)
			os.Exit(1)
		}
	}
	// retryableCommands lists the commands that may safely retry transient
	// failures. All listed commands issue only idempotent or read-only API calls.
	retryableCommands := map[string]bool{
		"push": true, "pull": true, "list-deployments": true, "clone": true,
	}
	retries := resolveHTTPRetries(cli.MaxRetries)
	if cli.NoRetries || !retryableCommands[commandName] {
		retries = 0
	}
	rc := &runContext{
		ctx:         context.Background(),
		httpTimeout: resolveHTTPTimeout(cli.Timeout, cli.NoTimeout),
		httpRetries: retries,
	}
	err := parsed.Run(rc)
	recordRunHistory(rawArgs, commandName, time.Since(start), err, rc.archiveMeta())
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// newLogHandler builds the slog handler for diagnostic logs. User-facing
// command output stays on the stdout/stderr writers via fmt and is never
// filtered by the log level.
func newLogHandler(w io.Writer, level, format string) slog.Handler {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if format == "json" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// resolveHTTPTimeout returns the HTTP timeout to use for Script API requests.
// Priority: --no-timeout > --timeout / GLASP_TIMEOUT env > .glasp/config.json > defaultHTTPTimeout.
// Returns 0 when noTimeout is true, which net/http interprets as no timeout.
// A value of 0 means "unset" (fall back to config/default); negative values
// are invalid and are warned about and ignored rather than silently dropped.
func resolveHTTPTimeout(flagSeconds int, noTimeout bool) time.Duration {
	if noTimeout {
		return 0
	}
	switch {
	case flagSeconds > 0:
		return time.Duration(flagSeconds) * time.Second
	case flagSeconds < 0:
		// 0 is the documented "unset" sentinel; a negative value is invalid
		// input. Warn so a mistaken --timeout/GLASP_TIMEOUT is visible.
		slog.Warn("--timeout/GLASP_TIMEOUT value is negative and ignored; using .glasp/config.json value or default",
			"value", flagSeconds, "default", defaultHTTPTimeout.String())
	default:
		// flagSeconds == 0 is the "unset" case: kong leaves an unset int flag
		// at its zero value, so 0 cannot mean a real timeout. Fall through to
		// the .glasp/config.json value, then the default. No warning.
	}
	projectRoot, err := config.FindProjectRoot()
	if err == nil && projectRoot != "" {
		glaspCfg, err := config.LoadGlaspConfig(projectRoot)
		switch {
		case err != nil:
			// LoadGlaspConfig returns an error only when the file exists but is
			// malformed/unreadable (a missing file yields a zero-value config).
			// Warn instead of silently honoring the default so a timeoutSeconds
			// typo is visible to the user rather than ignored.
			slog.Warn("failed to load .glasp/config.json; using default timeout",
				"default", defaultHTTPTimeout.String(), "error", err)
		case glaspCfg.TimeoutSeconds > 0:
			return time.Duration(glaspCfg.TimeoutSeconds) * time.Second
		case glaspCfg.TimeoutSeconds < 0:
			// Negative seconds are invalid; warn rather than silently default.
			slog.Warn("timeoutSeconds in .glasp/config.json is negative and ignored; using default timeout",
				"value", glaspCfg.TimeoutSeconds, "default", defaultHTTPTimeout.String())
		}
	}
	return defaultHTTPTimeout
}

// resolveHTTPRetries returns the number of retry attempts to use for Script API
// requests. Priority: --max-retries / GLASP_MAX_RETRIES > .glasp/config.json
// maxRetries > defaultHTTPRetries (3). A value of 0 means "unset"; negative
// values are invalid and are warned about and ignored.
func resolveHTTPRetries(flag int) int {
	switch {
	case flag > 0:
		return flag
	case flag < 0:
		slog.Warn("--max-retries/GLASP_MAX_RETRIES value is negative and ignored; using .glasp/config.json value or default",
			"value", flag, "default", defaultHTTPRetries)
	default:
		// flag == 0 is the "unset" case. Fall through to config/default.
	}
	projectRoot, err := config.FindProjectRoot()
	if err == nil && projectRoot != "" {
		glaspCfg, err := config.LoadGlaspConfig(projectRoot)
		switch {
		case err != nil:
			slog.Warn("failed to load .glasp/config.json; using default retries",
				"default", defaultHTTPRetries, "error", err)
		case glaspCfg.MaxRetries > 0:
			return glaspCfg.MaxRetries
		case glaspCfg.MaxRetries < 0:
			slog.Warn("maxRetries in .glasp/config.json is negative and ignored; using default retries",
				"value", glaspCfg.MaxRetries, "default", defaultHTTPRetries)
		}
	}
	return defaultHTTPRetries
}

// selectedCommandName returns the canonical command path that kong selected,
// e.g. "push" or "config init". It walks the parsed context path and keeps
// only command nodes, so global flags, their values, and positional argument
// values never leak into the recorded history command. Command aliases such as
// "deploy" or "open" resolve to their canonical names ("update-deployment",
// "open-script") because kong records the matched command, not the typed alias.
func selectedCommandName(ctx *kong.Context) string {
	if ctx == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	for _, p := range ctx.Path {
		if p.Command != nil {
			parts = append(parts, p.Command.Name)
		}
	}
	return strings.Join(parts, " ")
}

func recordRunHistory(args []string, commandName string, duration time.Duration, runErr error, archiveMeta runArchiveMeta) {
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		slog.Warn("failed to resolve project root for history", "error", err)
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
		slog.Warn("failed to append history entry", "error", err)
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
