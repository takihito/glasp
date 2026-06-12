package main

import (
	"fmt"

	"github.com/alecthomas/kong"
)

var (
	// Version is the semantic version. Overridden at build time via ldflags.
	Version = "v0.3.0"
	// Commit is the git commit hash. Overridden at build time via ldflags.
	Commit = "none"
	// Date is the build date. Overridden at build time via ldflags.
	Date = "unknown"
)

// VersionCmd represents the 'version' subcommand.
type VersionCmd struct{}

// Run executes the version command.
func (c *VersionCmd) Run(ctx *kong.Context) error {
	fmt.Printf("glasp version %s (commit=%s, date=%s)\n", Version, Commit, Date)
	return nil
}
