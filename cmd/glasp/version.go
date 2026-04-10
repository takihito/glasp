package main

var (
	// Version is the semantic version. Overridden at build time via ldflags.
	Version = "v0.1.2"
	// Commit is the git commit hash. Overridden at build time via ldflags.
	Commit = "none"
	// Date is the build date. Overridden at build time via ldflags.
	Date = "unknown"
)
