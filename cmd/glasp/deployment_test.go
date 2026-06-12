package main

import (
	"context"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"google.golang.org/api/script/v1"
)

func TestDeployAliasParsesAsUpdateDeployment(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"deploy", "dep-id", "--versionNumber", "8"})
	if err != nil {
		t.Fatalf("expected deploy alias to parse, got %v", err)
	}
	if cli.UpdateDeployment.DeploymentID != "dep-id" {
		t.Fatalf("expected deployment id dep-id, got %q", cli.UpdateDeployment.DeploymentID)
	}
	if cli.UpdateDeployment.Version != 8 {
		t.Fatalf("expected version 8, got %d", cli.UpdateDeployment.Version)
	}
}

func TestCreateDeploymentCLIParsesOptions(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"create-deployment", "--versionNumber", "4", "--description", "release", "--deploymentId", "dep-id"})
	if err != nil {
		t.Fatalf("expected create-deployment to parse, got %v", err)
	}
	if cli.CreateDeployment.Version != 4 {
		t.Fatalf("expected version 4, got %d", cli.CreateDeployment.Version)
	}
	if cli.CreateDeployment.Description != "release" {
		t.Fatalf("expected description release, got %q", cli.CreateDeployment.Description)
	}
	if cli.CreateDeployment.DeploymentID != "dep-id" {
		t.Fatalf("expected deploymentId dep-id, got %q", cli.CreateDeployment.DeploymentID)
	}
}

func TestListDeploymentsCLIParsesOptionalScriptID(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"list-deployments", "script-id"})
	if err != nil {
		t.Fatalf("expected list-deployments to parse, got %v", err)
	}
	if cli.ListDeployments.ScriptID != "script-id" {
		t.Fatalf("expected script-id, got %q", cli.ListDeployments.ScriptID)
	}
}

func TestUpdateDeploymentCommandFlowWithVersion(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 7,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "dep-id", Version: 7, Description: "hotfix"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("UpdateDeploymentCmd.Run failed: %v", err)
	}
	if fake.createVersionCalled {
		t.Fatalf("did not expect CreateVersion to be called when --versionNumber is provided")
	}
	if !fake.updateDeployCalled {
		t.Fatalf("expected UpdateDeployment to be called")
	}
	if fake.updateDeployConfig == nil || fake.updateDeployConfig.VersionNumber != 7 {
		t.Fatalf("unexpected deployment config: %#v", fake.updateDeployConfig)
	}
}

func TestUpdateDeploymentCommandFlowCreatesVersionWhenMissing(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		createVersionResp: &script.Version{VersionNumber: 9},
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 9,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "dep-id", Description: "release"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("UpdateDeploymentCmd.Run failed: %v", err)
	}
	if !fake.createVersionCalled {
		t.Fatalf("expected CreateVersion to be called when --versionNumber is not provided")
	}
	if fake.createVersionID != "script-id" {
		t.Fatalf("expected script-id for version creation, got %s", fake.createVersionID)
	}
	if fake.updateDeployConfig == nil || fake.updateDeployConfig.VersionNumber != 9 {
		t.Fatalf("unexpected deployment config: %#v", fake.updateDeployConfig)
	}
}

func TestUpdateDeploymentCommandRejectsMissingDeploymentID(t *testing.T) {
	useTempDir(t)
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	cmd := UpdateDeploymentCmd{}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected error for missing deployment ID")
	}
	if !strings.Contains(err.Error(), "deployment ID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDeploymentCommandRejectsBlankDeploymentID(t *testing.T) {
	useTempDir(t)
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		t.Fatal("expected to fail before client creation")
		return nil, nil
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "   "}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected error for blank deployment ID")
	}
	if !strings.Contains(err.Error(), "deployment ID is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDeploymentCommandFailsWhenEntryPointsMarshalFails(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 7,
			},
		},
	}
	origClient := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = origClient })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}
	origMarshal := marshalJSONFn
	t.Cleanup(func() { marshalJSONFn = origMarshal })
	marshalJSONFn = func(v any) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}

	cmd := UpdateDeploymentCmd{DeploymentID: "dep-id", Version: 7}
	err := cmd.Run(nil)
	if err == nil {
		t.Fatalf("expected error when entryPoints marshal fails")
	}
	if !strings.Contains(err.Error(), "failed to marshal deployment entry points") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateDeploymentCommandFlowCreatesNewDeployment(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		createVersionResp: &script.Version{VersionNumber: 11},
		createDeployResp: &script.Deployment{
			DeploymentId: "dep-new",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 11,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := CreateDeploymentCmd{Description: "release"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CreateDeploymentCmd.Run failed: %v", err)
	}
	if !fake.createVersionCalled {
		t.Fatalf("expected CreateVersion to be called when version is omitted")
	}
	if !fake.createDeployCalled {
		t.Fatalf("expected CreateDeployment to be called")
	}
	if fake.createDeployConfig == nil || fake.createDeployConfig.VersionNumber != 11 {
		t.Fatalf("unexpected deployment config: %#v", fake.createDeployConfig)
	}
}

func TestCreateDeploymentCommandWithDeploymentIDUsesUpdate(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		updateDeployResp: &script.Deployment{
			DeploymentId: "dep-id",
			DeploymentConfig: &script.DeploymentConfig{
				VersionNumber: 7,
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	cmd := CreateDeploymentCmd{DeploymentID: "dep-id", Version: 7, Description: "redeploy"}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("CreateDeploymentCmd.Run failed: %v", err)
	}
	if fake.createDeployCalled {
		t.Fatalf("did not expect CreateDeployment when deploymentId is provided")
	}
	if !fake.updateDeployCalled {
		t.Fatalf("expected UpdateDeployment to be called")
	}
}

func TestListDeploymentsCommandUsesConfigScriptID(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	fake := &fakeScriptClient{
		listDeployResp: []*script.Deployment{
			{
				DeploymentId: "dep-1",
				DeploymentConfig: &script.DeploymentConfig{
					VersionNumber: 2,
				},
			},
		},
	}
	orig := newScriptClientWithCacheAuthFn
	t.Cleanup(func() { newScriptClientWithCacheAuthFn = orig })
	newScriptClientWithCacheAuthFn = func(ctx context.Context, cachePath, authPath string) (scriptClient, error) {
		return fake, nil
	}

	if err := (&ListDeploymentsCmd{}).Run(nil); err != nil {
		t.Fatalf("ListDeploymentsCmd.Run failed: %v", err)
	}
	if !fake.listDeployCalled {
		t.Fatalf("expected ListDeployments to be called")
	}
	if fake.listDeployScriptID != "script-id" {
		t.Fatalf("expected script-id, got %s", fake.listDeployScriptID)
	}
}
