package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"glasp/internal/auth" // Import the auth package
	"glasp/internal/config"
	"glasp/internal/history"
	"glasp/internal/scriptapi"
	"glasp/internal/syncer"
	"glasp/internal/transform"

	"github.com/alecthomas/kong"
	"google.golang.org/api/script/v1"
)

type scriptClient interface {
	CreateProject(ctx context.Context, title, parentID string) (*script.Project, error)
	GetProject(ctx context.Context, scriptID string) (*script.Project, error)
	GetContent(ctx context.Context, scriptID string, versionNumber int64) (*script.Content, error)
	UpdateContent(ctx context.Context, scriptID string, content *script.Content) (*script.Content, error)
	CreateVersion(ctx context.Context, scriptID, description string) (*script.Version, error)
	CreateDeployment(ctx context.Context, scriptID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error)
	UpdateDeployment(ctx context.Context, scriptID, deploymentID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error)
	ListDeployments(ctx context.Context, scriptID string) ([]*script.Deployment, error)
	RunFunction(ctx context.Context, scriptID, functionName string, params []any, devMode bool) (*script.Operation, error)
}

var (
	newScriptClientWithCacheAuthFn = newScriptClientWithCachePathAndAuth
	transformConvertFn             = transform.Convert
	convertPulledContentFn         = convertPulledContent
	openURLFn                      = openURL
	execCommandFn                  = exec.Command
	runtimeGOOS                    = runtime.GOOS
	marshalJSONFn                  = json.Marshal
)

type runArchiveMeta struct {
	Enabled   bool
	Direction string
	Path      string
}

var currentRunArchive runArchiveMeta

const maxTitleLength = 256

var scriptIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// CLI is the main command-line interface structure for glasp.
type CLI struct {
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

// LoginCmd represents the 'login' subcommand.
type LoginCmd struct {
	Auth string `help:"Path to .clasprc.json to import as login credentials."`
}

// Run executes the login command.
func (c *LoginCmd) Run(ctx *kong.Context) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	cfgPath := filepath.Join(projectRoot, ".clasp.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := config.SaveClaspConfig(projectRoot, &config.ClaspConfig{}); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	authPath := strings.TrimSpace(c.Auth)
	if authPath != "" {
		cacheFile := filepath.Join(projectRoot, ".glasp", "access.json")
		if err := auth.ImportAuthFile(authPath, cacheFile); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		fmt.Println("Login successful.")
		return nil
	}

	oauthConfig, err := auth.Config()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	_, err = auth.Login(context.Background(), oauthConfig)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	fmt.Println("Login successful.")
	return nil
}

// LogoutCmd represents the 'logout' subcommand.
type LogoutCmd struct{}

// Run executes the logout command.
func (c *LogoutCmd) Run(ctx *kong.Context) error {
	err := auth.Logout()
	if err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}
	fmt.Println("Logout successful.")
	return nil
}

// CreateCmd represents the 'create' subcommand.
type CreateCmd struct {
	Title         string `help:"Title of the Apps Script project."`
	Type          string `help:"Type of the Apps Script project (e.g., standalone, sheet, doc)." default:"standalone"`
	RootDir       string `name:"rootDir" help:"Root directory for project files." default:"./"`
	ParentID      string `name:"parentId" help:"Parent Drive file ID for container-bound scripts."`
	FileExtension string `help:"Script file extension (e.g., js, gs, ts)." default:"js"`
	Auth          string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the create command.
func (c *CreateCmd) Run(ctx *kong.Context) error {
	title, err := validateTitle(c.Title)
	if err != nil {
		return err
	}
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	projectType := strings.ToLower(strings.TrimSpace(c.Type))
	if projectType == "" {
		projectType = "standalone"
	}
	if err := validateCreateType(projectType); err != nil {
		return err
	}
	parentID := strings.TrimSpace(c.ParentID)
	if parentID == "" && projectType != "standalone" {
		return fmt.Errorf("project type %q is not supported yet; currently only \"standalone\" is supported without --parentId", projectType)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	if err := ensureNoExistingClaspConfig(projectRoot); err != nil {
		return err
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}

	project, err := client.CreateProject(context.Background(), title, parentID)
	if err != nil {
		return err
	}
	cfg := &config.ClaspConfig{
		ScriptID: project.ScriptId,
		ParentID: project.ParentId,
	}
	cfg.RootDir = strings.TrimSpace(c.RootDir)
	if cfg.RootDir == "" {
		cfg.RootDir = "./"
	}
	fileExt := strings.TrimSpace(c.FileExtension)
	if fileExt != "" {
		fileExt = strings.TrimPrefix(strings.ToLower(fileExt), ".")
		if err := validateSupportedSyncFileExtension(fileExt); err != nil {
			return err
		}
		cfg.Extra = map[string]json.RawMessage{
			"fileExtension": json.RawMessage(fmt.Sprintf("%q", fileExt)),
		}
	}
	if err := config.SaveClaspConfig(projectRoot, cfg); err != nil {
		return err
	}

	content, err := client.GetContent(context.Background(), project.ScriptId, 0)
	if err != nil {
		return err
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, nil)
	if err != nil {
		return err
	}
	if _, err := syncer.ApplyRemoteContent(opts, content); err != nil {
		return err
	}

	fmt.Printf("Created project %s\n", project.ScriptId)
	return nil
}

// OpenScriptCmd represents the 'open-script' subcommand.
type OpenScriptCmd struct {
	ScriptID string `name:"scriptId" help:"Script ID override. If omitted, uses .clasp.json."`
}

// Run executes the open-script command.
func (c *OpenScriptCmd) Run(ctx *kong.Context) error {
	scriptID := strings.TrimSpace(c.ScriptID)
	if scriptID == "" {
		projectRoot, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine project root: %w", err)
		}
		cfg, err := config.LoadClaspConfig(projectRoot)
		if err != nil {
			return err
		}
		scriptID = cfg.ScriptID
	}
	validatedScriptID, err := validateScriptID(scriptID)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://script.google.com/d/%s/edit", validatedScriptID)
	if err := openURLFn(url); err != nil {
		fmt.Println(url)
		return fmt.Errorf("failed to open browser: %w", err)
	}
	fmt.Printf("Opened %s\n", url)
	return nil
}

// CreateDeploymentCmd represents the 'create-deployment' subcommand.
type CreateDeploymentCmd struct {
	Version      int64  `name:"versionNumber" short:"V" help:"Version number to deploy."`
	Description  string `name:"description" short:"d" help:"Deployment version description."`
	DeploymentID string `name:"deploymentId" short:"i" help:"Deployment ID to redeploy."`
	Auth         string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the create-deployment command.
func (c *CreateDeploymentCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	scriptID, err := scriptIDFromConfig(projectRoot)
	if err != nil {
		return err
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	versionNumber := c.Version
	if versionNumber <= 0 {
		version, err := client.CreateVersion(context.Background(), scriptID, strings.TrimSpace(c.Description))
		if err != nil {
			return err
		}
		versionNumber = version.VersionNumber
	}
	deployConfig := &script.DeploymentConfig{
		Description:      strings.TrimSpace(c.Description),
		ManifestFileName: "appsscript",
		VersionNumber:    versionNumber,
	}

	deploymentID := strings.TrimSpace(c.DeploymentID)
	var deployment *script.Deployment
	if deploymentID == "" {
		deployment, err = client.CreateDeployment(context.Background(), scriptID, deployConfig)
		if err != nil {
			return err
		}
		fmt.Printf("Created deployment %s (version=%d)\n", deployment.DeploymentId, deployConfig.VersionNumber)
	} else {
		deployment, err = client.UpdateDeployment(context.Background(), scriptID, deploymentID, deployConfig)
		if err != nil {
			return err
		}
		fmt.Printf("Updated deployment %s (version=%d)\n", deployment.DeploymentId, deployConfig.VersionNumber)
	}
	entryPointsJSON, err := marshalJSONFn(deployment.EntryPoints)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment entry points: %w", err)
	}
	fmt.Printf("entryPoints=%s\n", string(entryPointsJSON))
	for _, entry := range deployment.EntryPoints {
		if entry != nil && entry.WebApp != nil && strings.TrimSpace(entry.WebApp.Url) != "" {
			fmt.Printf("webAppUrl=%s\n", entry.WebApp.Url)
		}
	}
	return nil
}

// UpdateDeploymentCmd represents the 'update-deployment' subcommand.
type UpdateDeploymentCmd struct {
	DeploymentID string `arg:"" help:"Deployment ID to update."`
	Version      int64  `name:"versionNumber" short:"V" help:"Version number to deploy."`
	Description  string `name:"description" short:"d" help:"Deployment version description."`
	Auth         string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the update-deployment command.
func (c *UpdateDeploymentCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	deploymentID := strings.TrimSpace(c.DeploymentID)
	if deploymentID == "" {
		return fmt.Errorf("deployment ID is required")
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ScriptID) == "" {
		return fmt.Errorf("script ID is required in .clasp.json")
	}
	scriptID, err := validateScriptID(cfg.ScriptID)
	if err != nil {
		return err
	}

	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	versionNumber := c.Version
	if versionNumber <= 0 {
		version, err := client.CreateVersion(context.Background(), scriptID, strings.TrimSpace(c.Description))
		if err != nil {
			return err
		}
		versionNumber = version.VersionNumber
	}

	deployConfig := &script.DeploymentConfig{
		Description:      strings.TrimSpace(c.Description),
		ManifestFileName: "appsscript",
		VersionNumber:    versionNumber,
	}
	deployment, err := client.UpdateDeployment(context.Background(), scriptID, deploymentID, deployConfig)
	if err != nil {
		return err
	}
	entryPointsJSON, err := marshalJSONFn(deployment.EntryPoints)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment entry points: %w", err)
	}
	fmt.Printf("Updated deployment %s (version=%d)\n", deployment.DeploymentId, deployConfig.VersionNumber)
	fmt.Printf("entryPoints=%s\n", string(entryPointsJSON))
	for _, entry := range deployment.EntryPoints {
		if entry != nil && entry.WebApp != nil && strings.TrimSpace(entry.WebApp.Url) != "" {
			fmt.Printf("webAppUrl=%s\n", entry.WebApp.Url)
		}
	}
	return nil
}

// ListDeploymentsCmd represents the 'list-deployments' subcommand.
type ListDeploymentsCmd struct {
	ScriptID string `arg:"" optional:"" help:"Script ID override. If omitted, uses .clasp.json."`
	Auth     string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the list-deployments command.
func (c *ListDeploymentsCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	scriptID := strings.TrimSpace(c.ScriptID)
	if scriptID == "" {
		scriptID, err = scriptIDFromConfig(projectRoot)
		if err != nil {
			return err
		}
	} else {
		scriptID, err = validateScriptID(scriptID)
		if err != nil {
			return err
		}
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	deployments, err := client.ListDeployments(context.Background(), scriptID)
	if err != nil {
		return err
	}
	if len(deployments) == 0 {
		fmt.Println("No deployments found.")
		return nil
	}
	for _, dep := range deployments {
		if dep == nil {
			continue
		}
		versionNumber := int64(0)
		description := ""
		if dep.DeploymentConfig != nil {
			versionNumber = dep.DeploymentConfig.VersionNumber
			description = dep.DeploymentConfig.Description
		}
		fmt.Printf("deploymentId=%s version=%d description=%q\n", dep.DeploymentId, versionNumber, description)
		entryPointsJSON, err := marshalJSONFn(dep.EntryPoints)
		if err != nil {
			return fmt.Errorf("failed to marshal deployment entry points: %w", err)
		}
		fmt.Printf("entryPoints=%s\n", string(entryPointsJSON))
		for _, entry := range dep.EntryPoints {
			if entry != nil && entry.WebApp != nil && strings.TrimSpace(entry.WebApp.Url) != "" {
				fmt.Printf("webAppUrl=%s\n", entry.WebApp.Url)
			}
		}
	}
	return nil
}

// RunFunctionCmd represents the 'run-function' subcommand.
type RunFunctionCmd struct {
	FunctionName string `arg:"" help:"Function name to run."`
	NonDev       bool   `name:"nondev" help:"Run in non-dev mode (devMode=false)."`
	Params       string `name:"params" short:"p" help:"A JSON string array of parameters."`
	Auth         string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the run-function command.
func (c *RunFunctionCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	functionName := strings.TrimSpace(c.FunctionName)
	if functionName == "" {
		return fmt.Errorf("function name is required")
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	scriptID, err := scriptIDFromConfig(projectRoot)
	if err != nil {
		return err
	}
	params, err := parseRunParams(c.Params)
	if err != nil {
		return err
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	op, err := client.RunFunction(context.Background(), scriptID, functionName, params, !c.NonDev)
	if err != nil {
		return err
	}
	if op == nil {
		return fmt.Errorf("empty execution response")
	}
	if !op.Done {
		return fmt.Errorf("script execution is still in progress")
	}
	if op.Error != nil {
		message := strings.TrimSpace(op.Error.Message)
		if message != "" || op.Error.Code != 0 {
			return fmt.Errorf("script execution failed: code=%d message=%s", op.Error.Code, message)
		}
		return fmt.Errorf("script execution failed")
	}
	if len(op.Response) == 0 {
		fmt.Println("{}")
		return nil
	}
	fmt.Printf("%s\n", string(op.Response))
	return nil
}

// CloneCmd represents the 'clone' subcommand.
type CloneCmd struct {
	ScriptID      string `arg:"" help:"Script ID of the Apps Script project to clone."`
	RootDir       string `name:"rootDir" help:"Root directory for cloned project files." default:"./"`
	FileExtension string `help:"Script file extension for cloned files (e.g., js, gs, ts)." default:"js"`
	Auth          string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the clone command.
func (c *CloneCmd) Run(ctx *kong.Context) error {
	scriptID, err := validateScriptID(c.ScriptID)
	if err != nil {
		return err
	}
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	fileExt := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(c.FileExtension)), ".")
	if fileExt == "" {
		fileExt = "js"
	}
	if err := validateSupportedSyncFileExtension(fileExt); err != nil {
		return err
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	if err := ensureNoExistingClaspConfig(projectRoot); err != nil {
		return err
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	project, err := client.GetProject(context.Background(), scriptID)
	if err != nil {
		return err
	}
	cfg := &config.ClaspConfig{
		ScriptID: scriptID,
		ParentID: project.ParentId,
	}
	cfg.RootDir = strings.TrimSpace(c.RootDir)
	if cfg.RootDir == "" {
		cfg.RootDir = "./"
	}
	cfg.Extra = map[string]json.RawMessage{
		"fileExtension": json.RawMessage(fmt.Sprintf("%q", fileExt)),
	}
	if err := config.SaveClaspConfig(projectRoot, cfg); err != nil {
		return err
	}
	content, err := client.GetContent(context.Background(), scriptID, 0)
	if err != nil {
		return err
	}
	fileExtension := claspFileExtension(cfg)
	workingContent := content
	if isTypeScriptFileExtension(fileExtension) {
		workingContent, err = convertPulledContentFn(content, projectRoot)
		if err != nil {
			return err
		}
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, nil)
	if err != nil {
		return err
	}
	if _, err := syncer.ApplyRemoteContent(opts, workingContent); err != nil {
		return err
	}

	fmt.Printf("Cloned project %s\n", scriptID)
	return nil
}

// PullCmd represents the 'pull' subcommand.
type PullCmd struct {
	Archive bool   `help:"Save pulled files under .glasp/archive/<scriptId>/pull/YYYYMMDD_HHMMSS."`
	DryRun  bool   `name:"dryrun" help:"Run conversion planning only without API calls or local writes."`
	Auth    string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the pull command.
func (c *PullCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ScriptID) == "" {
		return fmt.Errorf("script ID is required in .clasp.json")
	}
	fileExtension := claspFileExtension(cfg)
	if err := validateSupportedSyncFileExtension(fileExtension); err != nil {
		return err
	}
	scriptID, err := validateScriptID(cfg.ScriptID)
	if err != nil {
		return err
	}
	glaspCfg, err := config.LoadGlaspConfig(projectRoot)
	if err != nil {
		return err
	}
	archiveEnabled := c.Archive
	if glaspCfg != nil && glaspCfg.Archive.Pull {
		archiveEnabled = true
	}
	setRunArchiveMeta(archiveEnabled, "pull")
	if c.DryRun {
		fmt.Printf("Dry run pull for project %s (convert=%s): skipped API fetch and local file writes\n", scriptID, dryRunConvertLabelForPull(fileExtension))
		return nil
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	content, err := client.GetContent(context.Background(), scriptID, 0)
	if err != nil {
		return err
	}
	pullMode := transform.Mode("")
	workingContent := content
	if isTypeScriptFileExtension(fileExtension) {
		pullMode = transform.ModeGasToTS
		workingContent, err = convertPulledContentFn(content, projectRoot)
		if err != nil {
			return err
		}
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, nil)
	if err != nil {
		return err
	}
	if archiveEnabled {
		archiveRoot, err := archivePullRun(projectRoot, scriptID, cfg, content, workingContent, fileExtension, pullMode)
		if err != nil {
			return err
		}
		setRunArchivePath(archiveRoot)
		fmt.Printf("Archived pull to %s\n", archiveRoot)
	}
	if _, err := syncer.ApplyRemoteContent(opts, workingContent); err != nil {
		return err
	}

	fmt.Printf("Pulled project %s\n", scriptID)
	return nil
}

// ConfigCmd represents the 'config' subcommand.
type ConfigCmd struct {
	Init ConfigInitCmd `cmd:"" help:"Create .glasp/config.json."`
}

// ConfigInitCmd represents the 'config init' subcommand.
type ConfigInitCmd struct{}

// Run executes the config init command.
func (c *ConfigInitCmd) Run(ctx *kong.Context) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	configPath := filepath.Join(projectRoot, ".glasp", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config already exists: %s", configPath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check %s: %w", configPath, err)
	}
	cfg := &config.GlaspConfig{
		Archive: config.ArchiveConfig{
			Pull: false,
			Push: false,
		},
	}
	if err := config.SaveGlaspConfig(projectRoot, cfg); err != nil {
		return err
	}
	fmt.Printf("Created config at %s\n", configPath)
	return nil
}

// PushCmd represents the 'push' subcommand.
type PushCmd struct {
	Watch     bool   `help:"Watch for local file changes and push automatically. TODO implement"`
	Force     bool   `help:"Force push all files, ignoring .claspignore and default excludes (use with caution). By default node_modules/ is excluded even without .claspignore."`
	Archive   bool   `help:"Save pushed files under .glasp/archive/<scriptId>/push/YYYYMMDD_HHMMSS."`
	DryRun    bool   `name:"dryrun" help:"Run conversion only without API calls or remote update."`
	HistoryID int64  `name:"history-id" help:"Push using payload archived by the specified history ID."`
	Auth      string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the push command.
func (c *PushCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	if c.Watch {
		return fmt.Errorf("watch mode is not implemented yet")
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine project root: %w", err)
	}
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ScriptID) == "" {
		return fmt.Errorf("script ID is required in .clasp.json")
	}
	fileExtension := claspFileExtension(cfg)
	if err := validateSupportedSyncFileExtension(fileExtension); err != nil {
		return err
	}
	scriptID, err := validateScriptID(cfg.ScriptID)
	if err != nil {
		return err
	}
	glaspCfg, err := config.LoadGlaspConfig(projectRoot)
	if err != nil {
		return err
	}
	archiveEnabled := c.Archive
	if glaspCfg != nil && glaspCfg.Archive.Push {
		archiveEnabled = true
	}
	setRunArchiveMeta(archiveEnabled, "push")
	if c.HistoryID < 0 {
		return fmt.Errorf("history-id must be >= 0")
	}
	if c.HistoryID > 0 {
		if c.Force {
			return fmt.Errorf("--history-id cannot be combined with --force")
		}
		return c.runFromHistoryID(projectRoot, scriptID, authPath, archiveEnabled, cfg.RootDir)
	}

	var ignore *config.ClaspIgnore
	if !c.Force {
		ignore, err = config.NewClaspIgnore(projectRoot)
		if err != nil {
			return err
		}
	}
	opts, err := syncer.OptionsFromConfig(projectRoot, cfg, ignore)
	if err != nil {
		return err
	}
	files, err := syncer.CollectLocalFiles(opts)
	if err != nil {
		return err
	}
	syncer.SortFilesByPushOrder(files, opts.FilePushOrder, opts.RootDir)
	// Enable TS→GAS transpilation when fileExtension is "ts" or when
	// collected files contain TypeScript sources.
	pushMode := transform.Mode("")
	if isTypeScriptFileExtension(fileExtension) || hasTypeScriptFiles(files) {
		pushMode = transform.ModeTSToGas
	}
	payloadFiles := files
	if pushMode == transform.ModeTSToGas {
		payloadFiles, err = convertPushFiles(files, projectRoot)
		if err != nil {
			return err
		}
	}
	content := syncer.BuildContent(payloadFiles)
	if c.DryRun {
		fmt.Printf("Dry run push for project %s (convert=%s): prepared %d files, skipped API update and archive writes\n", scriptID, convertLabel(pushMode), len(content.Files))
		return nil
	}

	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	if _, err := client.UpdateContent(context.Background(), scriptID, content); err != nil {
		return err
	}
	if archiveEnabled {
		archiveRoot, err := archivePushRun(projectRoot, scriptID, files, payloadFiles, fileExtension, pushMode)
		if err != nil {
			return err
		}
		setRunArchivePath(archiveRoot)
		fmt.Printf("Archived push to %s\n", archiveRoot)
	}

	fmt.Printf("Pushed project %s\n", scriptID)
	return nil
}

func (c *PushCmd) runFromHistoryID(projectRoot, scriptID, authPath string, archiveEnabled bool, rootDir string) error {
	entry, found, err := history.GetByID(projectRoot, c.HistoryID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("history id %d not found", c.HistoryID)
	}
	if !entry.Archive.Enabled {
		return fmt.Errorf("history id %d has archive disabled", c.HistoryID)
	}
	if cmd := strings.TrimSpace(entry.Command); cmd != "" && cmd != "push" {
		return fmt.Errorf("history id %d command is %q (expected push)", c.HistoryID, cmd)
	}
	if strings.TrimSpace(entry.Archive.Direction) != "push" {
		return fmt.Errorf("history id %d is not a push archive entry", c.HistoryID)
	}
	archivePath := strings.TrimSpace(entry.Archive.Path)
	if archivePath == "" {
		return fmt.Errorf("history id %d has empty archive path", c.HistoryID)
	}

	manifest, payloadFiles, err := loadPushArchivePayload(archivePath, rootDir)
	if err != nil {
		return fmt.Errorf("failed to load archive for history id %d: %w", c.HistoryID, err)
	}
	if strings.TrimSpace(manifest.ScriptID) == "" {
		return fmt.Errorf("archive manifest scriptId is empty: %s", filepath.Join(archivePath, "manifest.json"))
	}
	if manifest.ScriptID != scriptID {
		return fmt.Errorf("archive scriptId mismatch: history has %s but current project is %s", manifest.ScriptID, scriptID)
	}
	fmt.Printf(
		"Using history source id=%d archive.direction=%s archive.path=%s manifest.timestamp=%s\n",
		c.HistoryID,
		entry.Archive.Direction,
		archivePath,
		manifest.Timestamp,
	)
	content := syncer.BuildContent(payloadFiles)
	if c.DryRun {
		fmt.Printf("Dry run push from history id %d for project %s: prepared %d files from %s, skipped API update\n", c.HistoryID, scriptID, len(content.Files), archivePath)
		return nil
	}

	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	if _, err := client.UpdateContent(context.Background(), scriptID, content); err != nil {
		return err
	}
	if archiveEnabled {
		archiveRoot, err := archivePushRun(projectRoot, scriptID, payloadFiles, payloadFiles, manifest.FileExtension, modeFromArchiveConvert(manifest.Convert))
		if err != nil {
			return err
		}
		setRunArchivePath(archiveRoot)
		fmt.Printf("Archived push to %s\n", archiveRoot)
	}
	fmt.Printf("Pushed project %s from history id %d\n", scriptID, c.HistoryID)
	return nil
}

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

// HistoryCmd represents the 'history' subcommand.
type HistoryCmd struct {
	Limit   int    `name:"limit" help:"Maximum number of entries to show." default:"20"`
	Status  string `name:"status" help:"Filter by status (all|success|error)." default:"all"`
	Command string `name:"command" help:"Filter by exact command name."`
	Order   string `name:"order" help:"Sort order (desc|asc)." default:"desc"`
}

// Run executes the history command.
func (c *HistoryCmd) Run(ctx *kong.Context) error {
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	if projectRoot == "" {
		return fmt.Errorf(".clasp.json not found in current or parent directories")
	}
	entries, err := history.Read(projectRoot, history.ReadOptions{
		Limit:   c.Limit,
		Status:  strings.TrimSpace(c.Status),
		Command: strings.TrimSpace(c.Command),
		Order:   strings.TrimSpace(c.Order),
	})
	if err != nil {
		return err
	}
	data, err := marshalJSONFn(entries)
	if err != nil {
		return fmt.Errorf("failed to marshal history entries: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// VersionCmd represents the 'version' subcommand.
type VersionCmd struct{}

// Run executes the version command.
func (c *VersionCmd) Run(ctx *kong.Context) error {
	fmt.Printf("glasp version %s (commit=%s, date=%s)\n", Version, Commit, Date)
	return nil
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

func ensureNoExistingClaspConfig(projectRoot string) error {
	if projectRoot == "" {
		return fmt.Errorf("project root is empty")
	}
	cfgPath := filepath.Join(projectRoot, ".clasp.json")
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to check .clasp.json: %w", err)
	}
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ScriptID) != "" {
		return fmt.Errorf(".clasp.json already exists in %s", projectRoot)
	}
	return nil
}

func validateTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", fmt.Errorf("title is required")
	}
	if utf8.RuneCountInString(trimmed) > maxTitleLength {
		return "", fmt.Errorf("title too long (max %d characters)", maxTitleLength)
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("title contains invalid control characters")
		}
	}
	return trimmed, nil
}

func validateScriptID(scriptID string) (string, error) {
	trimmed := strings.TrimSpace(scriptID)
	if trimmed == "" {
		return "", fmt.Errorf("script ID is required")
	}
	if !scriptIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid script ID format")
	}
	return trimmed, nil
}

func scriptIDFromConfig(projectRoot string) (string, error) {
	cfg, err := config.LoadClaspConfig(projectRoot)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.ScriptID) == "" {
		return "", fmt.Errorf("script ID is required in .clasp.json")
	}
	return validateScriptID(cfg.ScriptID)
}

func parseRunParams(raw string) ([]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("params must be a JSON array: %w", err)
	}
	params, ok := decoded.([]any)
	if !ok {
		return nil, fmt.Errorf("params must be a JSON array")
	}
	return params, nil
}

func validateCreateType(projectType string) error {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "standalone", "webapp", "api", "docs", "sheets", "slides", "forms":
		return nil
	default:
		return fmt.Errorf("invalid project type %q", projectType)
	}
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
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileExtension)), ".")
}

func isTypeScriptFileExtension(fileExtension string) bool {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileExtension)), ".") {
	case "ts", "tsx":
		return true
	default:
		return false
	}
}

func isTSXFileExtension(fileExtension string) bool {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileExtension)), ".") == "tsx"
}

func validateSupportedSyncFileExtension(fileExtension string) error {
	if isTSXFileExtension(fileExtension) {
		return fmt.Errorf("fileExtension \"tsx\" is not supported for pull/push auto conversion; use \"ts\" or run convert manually")
	}
	return nil
}

func convertLabel(mode transform.Mode) string {
	switch mode {
	case transform.ModeGasToTS:
		return "gas-to-ts"
	case transform.ModeTSToGas:
		return "ts-to-gas"
	default:
		return "none"
	}
}

func dryRunConvertLabelForPull(fileExtension string) string {
	if isTypeScriptFileExtension(fileExtension) {
		return convertLabel(transform.ModeGasToTS)
	}
	return convertLabel("")
}

func optionalAuthPath(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("--auth path is empty")
	}
	return filepath.Clean(trimmed), nil
}

const fileTypeServerJSForConvert = "SERVER_JS"

func newScriptClientWithCachePathAndAuth(ctx context.Context, cacheFile, authPath string) (scriptClient, error) {
	return newScriptClientWithAuthInputs(ctx, cacheFile, authPath)
}

func newScriptClientWithAuthInputs(ctx context.Context, cacheFile, authPath string) (scriptClient, error) {
	var source auth.Source
	switch {
	case strings.TrimSpace(authPath) != "":
		source = auth.Source{
			Kind: auth.SourceKindAuthFile,
			Path: authPath,
		}
	case strings.TrimSpace(cacheFile) != "":
		source = auth.Source{
			Kind: auth.SourceKindProjectCache,
			Path: cacheFile,
		}
	default:
		oauthConfig, err := auth.Config()
		if err != nil {
			return nil, err
		}
		httpClient, err := auth.Login(ctx, oauthConfig)
		if err != nil {
			return nil, err
		}
		return scriptapi.New(ctx, httpClient)
	}

	httpClient, err := auth.EnsureAccessToken(ctx, source)
	if err != nil {
		return nil, err
	}
	return scriptapi.New(ctx, httpClient)
}

func newProjectScriptClient(ctx context.Context, projectRoot, authPath string) (scriptClient, error) {
	source, err := auth.ResolveAuthSource(projectRoot, authPath)
	if err != nil {
		return nil, err
	}
	if source.Kind == auth.SourceKindAuthFile {
		return newScriptClientWithCacheAuthFn(ctx, "", source.Path)
	}
	return newScriptClientWithCacheAuthFn(ctx, source.Path, "")
}

func openURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("url is empty")
	}
	var cmd *exec.Cmd
	switch runtimeGOOS {
	case "darwin":
		cmd = execCommandFn("open", url)
	case "linux":
		cmd = execCommandFn("xdg-open", url)
	case "windows":
		cmd = execCommandFn("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtimeGOOS)
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
