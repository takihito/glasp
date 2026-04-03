package scriptapi

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/option"
	"google.golang.org/api/script/v1"
)

// Client wraps the Google Apps Script API service.
type Client struct {
	service *script.Service
}

// New creates a new Script API client using the provided HTTP client.
func New(ctx context.Context, httpClient *http.Client, opts ...option.ClientOption) (*Client, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client is nil")
	}
	options := append([]option.ClientOption{option.WithHTTPClient(httpClient)}, opts...)
	service, err := script.NewService(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create script service: %w", err)
	}
	return &Client{service: service}, nil
}

// CreateProject creates a new Google Apps Script project.
func (c *Client) CreateProject(ctx context.Context, title, parentID string) (*script.Project, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	req := &script.CreateProjectRequest{Title: title}
	if parentID != "" {
		req.ParentId = parentID
	}
	project, err := c.service.Projects.Create(req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	return project, nil
}

// GetProject retrieves a Google Apps Script project by its script ID.
func (c *Client) GetProject(ctx context.Context, scriptID string) (*script.Project, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	project, err := c.service.Projects.Get(scriptID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return project, nil
}

// GetContent retrieves the content of a Google Apps Script project.
func (c *Client) GetContent(ctx context.Context, scriptID string, versionNumber int64) (*script.Content, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	call := c.service.Projects.GetContent(scriptID).Context(ctx)
	if versionNumber > 0 {
		call = call.VersionNumber(versionNumber)
	}
	content, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get project content: %w", err)
	}
	return content, nil
}

// UpdateContent updates the content of the Apps Script project identified by scriptID.
func (c *Client) UpdateContent(ctx context.Context, scriptID string, content *script.Content) (*script.Content, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	if content == nil {
		return nil, fmt.Errorf("content is required")
	}
	updated, err := c.service.Projects.UpdateContent(scriptID, content).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update project content: %w", err)
	}
	return updated, nil
}

// CreateVersion creates a new immutable version for the script project.
func (c *Client) CreateVersion(ctx context.Context, scriptID, description string) (*script.Version, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	version, err := c.service.Projects.Versions.Create(scriptID, &script.Version{
		Description: description,
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create project version: %w", err)
	}
	return version, nil
}

// CreateDeployment creates a deployment for the script project.
func (c *Client) CreateDeployment(ctx context.Context, scriptID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	if deploymentConfig == nil {
		return nil, fmt.Errorf("deploymentConfig is required")
	}
	deployment, err := c.service.Projects.Deployments.Create(scriptID, deploymentConfig).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}
	return deployment, nil
}

// UpdateDeployment updates an existing deployment for the script project.
func (c *Client) UpdateDeployment(ctx context.Context, scriptID, deploymentID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	if deploymentID == "" {
		return nil, fmt.Errorf("deploymentID is required")
	}
	if deploymentConfig == nil {
		return nil, fmt.Errorf("deploymentConfig is required")
	}
	deployment, err := c.service.Projects.Deployments.Update(scriptID, deploymentID, &script.UpdateDeploymentRequest{
		DeploymentConfig: deploymentConfig,
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment: %w", err)
	}
	return deployment, nil
}

// ListDeployments lists deployments for the script project.
func (c *Client) ListDeployments(ctx context.Context, scriptID string) ([]*script.Deployment, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	var (
		allDeployments []*script.Deployment
		pageToken      string
	)
	for {
		call := c.service.Projects.Deployments.List(scriptID).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list deployments: %w", err)
		}
		allDeployments = append(allDeployments, resp.Deployments...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return allDeployments, nil
}

// RunFunction runs an Apps Script function remotely.
func (c *Client) RunFunction(ctx context.Context, scriptID, functionName string, params []any, devMode bool) (*script.Operation, error) {
	if scriptID == "" {
		return nil, fmt.Errorf("scriptID is required")
	}
	if functionName == "" {
		return nil, fmt.Errorf("functionName is required")
	}
	op, err := c.service.Scripts.Run(scriptID, &script.ExecutionRequest{
		Function:   functionName,
		Parameters: params,
		DevMode:    devMode,
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to run function: %w", err)
	}
	return op, nil
}
