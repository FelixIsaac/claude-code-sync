package main

import (
	"os"

	"github.com/felixisaac/claude-code-sync/internal/cmd"
)

// Set by goreleaser ldflags
var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
