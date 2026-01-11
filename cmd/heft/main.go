package main

import (
	"fmt"
	"os"

	"github.com/tonur/heft/internal/cli"
)

// Version is the application version. It is set at build time via ldflags.
var Version = "dev"

// exitFunc is used by main to terminate the process. It is a variable
// so tests can stub it and observe exit behavior.
var exitFunc = os.Exit

// run executes the heft command with the provided arguments and returns
// an exit code.
func run(args []string) int {
	// Handle --version flags/args before Cobra so
	// version works regardless of subcommand or arg validation.
	for _, arg := range args {
		if arg == "--version" || arg == "-version" || arg == "version" {
			fmt.Println(Version)
			return 0
		}
	}

	cli.Execute()
	return 0
}

func main() {
	exitFunc(run(os.Args[1:]))
}
