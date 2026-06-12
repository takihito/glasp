// Package browser opens URLs in the platform default web browser.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Test seams.
var (
	execCommand = exec.Command
	goos        = runtime.GOOS
)

func command(url string) (*exec.Cmd, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("url is empty")
	}
	switch goos {
	case "darwin":
		return execCommand("open", url), nil
	case "linux":
		return execCommand("xdg-open", url), nil
	case "windows":
		return execCommand("rundll32", "url.dll,FileProtocolHandler", url), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}

// Open opens the URL and waits for the launcher command to finish,
// reporting its exit status as an error.
func Open(url string) error {
	cmd, err := command(url)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// Start opens the URL without waiting for the launcher command to finish.
// Use this when the caller must keep running (e.g. an OAuth callback server).
func Start(url string) error {
	cmd, err := command(url)
	if err != nil {
		return err
	}
	return cmd.Start()
}
