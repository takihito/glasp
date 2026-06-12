package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/takihito/glasp/internal/config"

	"github.com/alecthomas/kong"
	"google.golang.org/api/script/v1"
)

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
	projectRoot, err := findExistingProjectRoot()
	if err != nil {
		return err
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

	projectRoot, err := findExistingProjectRoot()
	if err != nil {
		return err
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
	projectRoot, err := findExistingProjectRoot()
	if err != nil {
		return err
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
