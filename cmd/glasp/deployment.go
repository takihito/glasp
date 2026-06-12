package main

import (
	"context"
	"fmt"
	"strings"

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
func (c *CreateDeploymentCmd) Run(rc *runContext) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	pc, err := loadProjectContext()
	if err != nil {
		return err
	}
	client, err := newProjectScriptClient(rc.Context(), pc.Root, authPath)
	if err != nil {
		return err
	}
	deployConfig, err := resolveDeploymentConfig(rc.Context(), client, pc.ScriptID, c.Version, c.Description)
	if err != nil {
		return err
	}

	deploymentID := strings.TrimSpace(c.DeploymentID)
	var deployment *script.Deployment
	if deploymentID == "" {
		deployment, err = client.CreateDeployment(rc.Context(), pc.ScriptID, deployConfig)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created deployment %s (version=%d)\n", deployment.DeploymentId, deployConfig.VersionNumber)
	} else {
		deployment, err = client.UpdateDeployment(rc.Context(), pc.ScriptID, deploymentID, deployConfig)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Updated deployment %s (version=%d)\n", deployment.DeploymentId, deployConfig.VersionNumber)
	}
	return printEntryPoints(deployment.EntryPoints)
}

// UpdateDeploymentCmd represents the 'update-deployment' subcommand.
type UpdateDeploymentCmd struct {
	DeploymentID string `arg:"" help:"Deployment ID to update."`
	Version      int64  `name:"versionNumber" short:"V" help:"Version number to deploy."`
	Description  string `name:"description" short:"d" help:"Deployment version description."`
	Auth         string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the update-deployment command.
func (c *UpdateDeploymentCmd) Run(rc *runContext) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	deploymentID := strings.TrimSpace(c.DeploymentID)
	if deploymentID == "" {
		return fmt.Errorf("deployment ID is required")
	}

	pc, err := loadProjectContext()
	if err != nil {
		return err
	}
	client, err := newProjectScriptClient(rc.Context(), pc.Root, authPath)
	if err != nil {
		return err
	}
	deployConfig, err := resolveDeploymentConfig(rc.Context(), client, pc.ScriptID, c.Version, c.Description)
	if err != nil {
		return err
	}
	deployment, err := client.UpdateDeployment(rc.Context(), pc.ScriptID, deploymentID, deployConfig)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Updated deployment %s (version=%d)\n", deployment.DeploymentId, deployConfig.VersionNumber)
	return printEntryPoints(deployment.EntryPoints)
}

// ListDeploymentsCmd represents the 'list-deployments' subcommand.
type ListDeploymentsCmd struct {
	ScriptID string `arg:"" optional:"" help:"Script ID override. If omitted, uses .clasp.json."`
	Auth     string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the list-deployments command.
func (c *ListDeploymentsCmd) Run(rc *runContext) error {
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
	client, err := newProjectScriptClient(rc.Context(), projectRoot, authPath)
	if err != nil {
		return err
	}
	deployments, err := client.ListDeployments(rc.Context(), scriptID)
	if err != nil {
		return err
	}
	if len(deployments) == 0 {
		fmt.Fprintln(stdout, "No deployments found.")
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
		fmt.Fprintf(stdout, "deploymentId=%s version=%d description=%q\n", dep.DeploymentId, versionNumber, description)
		if err := printEntryPoints(dep.EntryPoints); err != nil {
			return err
		}
	}
	return nil
}

// resolveDeploymentConfig builds the deployment config for the requested
// version, creating a new immutable version when none is specified.
func resolveDeploymentConfig(ctx context.Context, client scriptClient, scriptID string, versionNumber int64, description string) (*script.DeploymentConfig, error) {
	description = strings.TrimSpace(description)
	if versionNumber <= 0 {
		version, err := client.CreateVersion(ctx, scriptID, description)
		if err != nil {
			return nil, err
		}
		versionNumber = version.VersionNumber
	}
	return &script.DeploymentConfig{
		Description:      description,
		ManifestFileName: "appsscript",
		VersionNumber:    versionNumber,
	}, nil
}

// printEntryPoints prints a deployment's entry points as JSON plus one
// webAppUrl line per web app entry point.
func printEntryPoints(entryPoints []*script.EntryPoint) error {
	entryPointsJSON, err := marshalJSONFn(entryPoints)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment entry points: %w", err)
	}
	fmt.Fprintf(stdout, "entryPoints=%s\n", string(entryPointsJSON))
	for _, entry := range entryPoints {
		if entry != nil && entry.WebApp != nil && strings.TrimSpace(entry.WebApp.Url) != "" {
			fmt.Fprintf(stdout, "webAppUrl=%s\n", entry.WebApp.Url)
		}
	}
	return nil
}
