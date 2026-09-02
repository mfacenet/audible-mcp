// Package version holds build-time identity for the audible-mcp binary.
// GoReleaser and `task build` override these via -ldflags.
package version

// Version is the semantic version, or "dev" for local builds.
var Version = "dev"

// Commit is the short git commit, or "unknown" for local builds.
var Commit = "unknown"

// Date is the build timestamp, or "unknown" for local builds.
var Date = "unknown"
