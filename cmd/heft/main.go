package main

import (
	"fmt"
	"os"

	"github.com/tonur/heft/internal/cli"
)

// Version is the application version. It is set at build time via ldflags.
var Version = "dev"

// exitFunction is used by main to terminate the process. It is a variable
// so tests can stub it and observe exit behavior.
var exitFunction = os.Exit

// run executes the heft command with the provided arguments and returns
// an exit code.
func run(arguments []string) int {
	// Handle --version flags/arguments before Cobra so
	// version works regardless of subcommand or arg validation.
	for _, argument := range arguments {
		if argument == "--version" || argument == "-version" || argument == "version" {
			fmt.Println(Version)
			return 0
		}
	}

	cli.Execute()
	return 0
}

func main() {
	exitFunction(run(os.Args[1:]))
}
