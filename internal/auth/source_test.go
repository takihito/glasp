package auth

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"
)

func TestResolveAuthSourceAuthFile(t *testing.T) {
	src, err := ResolveAuthSource("/tmp/project", " ./auth/../auth/.clasprc.json ")
	if err != nil {
		t.Fatalf("ResolveAuthSource failed: %v", err)
	}
	if src.Kind != SourceKindAuthFile {
		t.Fatalf("expected auth file source, got %s", src.Kind)
	}
	expected := filepath.Clean("./auth/../auth/.clasprc.json")
	if src.Path != expected {
		t.Fatalf("expected cleaned auth path %q, got %q", expected, src.Path)
	}
}

func TestResolveAuthSourceProjectCache(t *testing.T) {
	root := t.TempDir()
	src, err := ResolveAuthSource(root, "")
	if err != nil {
		t.Fatalf("ResolveAuthSource failed: %v", err)
	}
	if src.Kind != SourceKindProjectCache {
		t.Fatalf("expected project cache source, got %s", src.Kind)
	}
	expected := filepath.Join(root, ".glasp", "access.json")
	if src.Path != expected {
		t.Fatalf("expected project cache path %q, got %q", expected, src.Path)
	}
}

func TestResolveAuthSourcePreservesProjectRootWhitespace(t *testing.T) {
	root := t.TempDir()
	rootWithSpaces := " " + root + " "
	src, err := ResolveAuthSource(rootWithSpaces, "")
	if err != nil {
		t.Fatalf("ResolveAuthSource failed: %v", err)
	}
	expected := filepath.Join(rootWithSpaces, ".glasp", "access.json")
	if src.Path != expected {
		t.Fatalf("expected whitespace-preserved path %q, got %q", expected, src.Path)
	}
}

func TestResolveAuthSourceRequiresProjectRootWithoutAuthPath(t *testing.T) {
	if _, err := ResolveAuthSource("   ", ""); err == nil {
		t.Fatalf("expected error for empty project root")
	}
}

func TestEnsureAccessTokenRejectsUnknownSourceKind(t *testing.T) {
	_, err := EnsureAccessToken(context.Background(), Source{
		Kind: SourceKind("unknown"),
		Path: "/tmp/token.json",
	})
	if err == nil {
		t.Fatalf("expected error for unknown source kind")
	}
}

func TestEnsureAccessTokenPreservesProjectCachePathWhitespace(t *testing.T) {
	origConfigFn := configFn
	origLoginFn := loginWithCachePathFn
	t.Cleanup(func() {
		configFn = origConfigFn
		loginWithCachePathFn = origLoginFn
	})

	configFn = func() (*oauth2.Config, error) {
		return &oauth2.Config{}, nil
	}

	var gotPath string
	loginWithCachePathFn = func(ctx context.Context, cfg *oauth2.Config, cacheFile string) (*http.Client, error) {
		gotPath = cacheFile
		return &http.Client{}, nil
	}

	sourcePath := " /tmp/project with spaces /.glasp/access.json "
	if _, err := EnsureAccessToken(context.Background(), Source{
		Kind: SourceKindProjectCache,
		Path: sourcePath,
	}); err != nil {
		t.Fatalf("EnsureAccessToken failed: %v", err)
	}
	if gotPath != sourcePath {
		t.Fatalf("expected project cache path %q, got %q", sourcePath, gotPath)
	}
}

func TestEnsureAccessTokenTrimsAuthFilePath(t *testing.T) {
	origAuthClientFn := clientFromAuthFileWithRefreshFn
	t.Cleanup(func() { clientFromAuthFileWithRefreshFn = origAuthClientFn })

	var gotPath string
	clientFromAuthFileWithRefreshFn = func(ctx context.Context, authPath string) (*http.Client, error) {
		gotPath = authPath
		return &http.Client{}, nil
	}

	if _, err := EnsureAccessToken(context.Background(), Source{
		Kind: SourceKindAuthFile,
		Path: " ./auth/.clasprc.json ",
	}); err != nil {
		t.Fatalf("EnsureAccessToken failed: %v", err)
	}
	if gotPath != "./auth/.clasprc.json" {
		t.Fatalf("expected trimmed auth path, got %q", gotPath)
	}
}
