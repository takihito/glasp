package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/takihito/glasp/internal/auth"
	"github.com/takihito/glasp/internal/config"

	"github.com/alecthomas/kong"
)

// LoginCmd represents the 'login' subcommand.
type LoginCmd struct {
	Auth string `help:"Path to .clasprc.json to import as login credentials."`
	PKCE bool   `name:"pkce" env:"GLASP_USE_PKCE" help:"Enable PKCE (Proof Key for Code Exchange) for the interactive OAuth login flow."`
}

// Run executes the login command.
// The project root is resolved the same way as other commands: the nearest
// directory containing .clasp.json, searching upward from the current
// directory. Only when no project exists yet is an empty .clasp.json
// created in the current directory.
func (c *LoginCmd) Run(ctx *kong.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if projectRoot == "" {
		projectRoot = cwd
		if err := config.SaveClaspConfig(projectRoot, &config.ClaspConfig{}); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	} else if filepath.Clean(projectRoot) != filepath.Clean(cwd) {
		fmt.Fprintf(stderr, "Project root: %s\n", projectRoot)
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
	_, err = auth.LoginWithOptions(context.Background(), oauthConfig, auth.LoginOptions{PKCE: c.PKCE})
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
