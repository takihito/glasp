package main

import (
	"fmt"
)

var (
	// Version is the semantic version. Overridden at build time via ldflags.
	Version = "v0.4.0"
	// Commit is the git commit hash. Overridden at build time via ldflags.
	Commit = "none"
	// Date is the build date. Overridden at build time via ldflags.
	Date = "unknown"
)

// VersionCmd represents the 'version' subcommand.
type VersionCmd struct{}

// Run executes the version command.
func (c *VersionCmd) Run(rc *runContext) error {
	fmt.Fprintf(stdout, "glasp version %s (commit=%s, date=%s)\n", Version, Commit, Date)
	return nil
}
