package main

import (
	"fmt"
	"os"

	"github.com/tonur/heft/internal/cli"
)

// Version is the application version. It is set at build time via ldflags.
var Version = "dev"

func main() {
	// Handle --version flags/args before Cobra so
	// version works regardless of subcommand or arg validation.
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" || arg == "version" {
			fmt.Println(Version)
			os.Exit(0)
		}
	}

	cli.Execute()
}
