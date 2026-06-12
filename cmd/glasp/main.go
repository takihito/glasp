package main

import (
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

var currentRunArchive runArchiveMeta

// CLI is the main command-line interface structure for glasp.
type CLI struct {
	Dir              string              `name:"dir" short:"C" env:"GLASP_DIR" help:"Change to this directory before executing any command."`
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
	resetRunArchiveMeta()
	rawArgs := append([]string(nil), os.Args[1:]...)
	commandName := commandFromArgs(rawArgs)

	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("glasp"),
		kong.Description("A Go-based Google Apps Script CLI (clasp alternative)."),
		kong.UsageOnError(),
	)
	if dir := strings.TrimSpace(cli.Dir); dir != "" {
		if err := os.Chdir(dir); err != nil {
			log.Fatalf("Error: failed to change directory to %q: %v", dir, err)
		}
	}
	err := ctx.Run(&cli)
	recordRunHistory(rawArgs, commandName, time.Since(start), err)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func commandFromArgs(args []string) string {
	var first string
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		first = arg
		break
	}
	if first == "" {
		return ""
	}

	// Keep aliases as entered, but include known nested subcommands
	// so `config init` is distinguishable from the command group itself.
	if first == "config" {
		foundFirst := false
		for _, arg := range args {
			if strings.TrimSpace(arg) == "" || strings.HasPrefix(arg, "-") {
				continue
			}
			if !foundFirst {
				foundFirst = true
				continue
			}
			return first + " " + arg
		}
	}
	return first
}

func recordRunHistory(args []string, commandName string, duration time.Duration, runErr error) {
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
			Enabled:   currentRunArchive.Enabled,
			Direction: currentRunArchive.Direction,
			Path:      currentRunArchive.Path,
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

func resetRunArchiveMeta() {
	currentRunArchive = runArchiveMeta{}
}

func setRunArchiveMeta(enabled bool, direction string) {
	currentRunArchive.Enabled = enabled
	currentRunArchive.Direction = direction
	currentRunArchive.Path = ""
}

func setRunArchivePath(path string) {
	currentRunArchive.Path = path
}
