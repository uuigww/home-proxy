// Package version holds the binary version, injected at build time via -ldflags.
package version

// Version is overridden in Makefile / GoReleaser via -X.
var Version = "dev"
