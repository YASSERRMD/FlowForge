// Package buildinfo exposes build metadata shared by the api and worker
// binaries.
package buildinfo

// Version is the human-readable service version. It is replaced by the
// release pipeline; during local development it stays "dev".
var Version = "dev"

// Commit is the Git commit hash injected at build time via -ldflags.
var Commit = "unknown"
