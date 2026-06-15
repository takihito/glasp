package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/takihito/glasp/internal/auth"
	"github.com/takihito/glasp/internal/scriptapi"

	"google.golang.org/api/script/v1"
)

type httpTimeoutCtxKey struct{}

func withHTTPTimeout(ctx context.Context, d time.Duration) context.Context {
	if d <= 0 {
		return ctx
	}
	return context.WithValue(ctx, httpTimeoutCtxKey{}, d)
}

func httpTimeoutFromCtx(ctx context.Context) time.Duration {
	d, _ := ctx.Value(httpTimeoutCtxKey{}).(time.Duration)
	return d
}

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
	case strings.TrimSpace(os.Getenv("GLASP_AUTH")) != "":
		source = auth.Source{
			Kind:    auth.SourceKindAuthJSON,
			Content: os.Getenv("GLASP_AUTH"),
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
		applyHTTPTimeout(ctx, httpClient)
		return scriptapi.New(ctx, httpClient)
	}

	httpClient, err := auth.EnsureAccessToken(ctx, source)
	if err != nil {
		return nil, err
	}
	applyHTTPTimeout(ctx, httpClient)
	return scriptapi.New(ctx, httpClient)
}

// applyHTTPTimeout sets the Timeout field of httpClient to the value stored in
// ctx by withHTTPTimeout. No-op when no timeout is present in ctx.
func applyHTTPTimeout(ctx context.Context, httpClient *http.Client) {
	if timeout := httpTimeoutFromCtx(ctx); timeout > 0 {
		httpClient.Timeout = timeout
	}
}

func newProjectScriptClient(ctx context.Context, projectRoot, authPath string, timeout time.Duration) (scriptClient, error) {
	ctx = withHTTPTimeout(ctx, timeout)
	source, err := auth.ResolveAuthSource(projectRoot, authPath)
	if err != nil {
		return nil, err
	}
	if source.Kind == auth.SourceKindAuthFile {
		return newScriptClientWithCacheAuthFn(ctx, "", source.Path)
	}
	return newScriptClientWithCacheAuthFn(ctx, source.Path, "")
}
