package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestRunVersionFlag ensures that the run helper handles the
// --version flag and returns a zero exit code without invoking exitFunction.
func TestRunVersionFlag(t *testing.T) {
	code := run([]string{"--version"})
	if code != 0 {
		t.Fatalf("expected exit code 0 from run, got %d", code)
	}
}

// TestMainVersionFlag ensures that the --version flag is handled by
// the main entrypoint and prints a non-empty version string.
func TestMainVersionFlag(t *testing.T) {
	// Run `go run` on the heft command from the repository root.
	command := exec.Command("go", "run", "./cmd/heft", "--version")
	command.Dir = "../.."

	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/heft --version failed: %v\noutput: %s", err, string(out))
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		t.Fatalf("expected non-empty version output, got %q", version)
	}
}

// TestMainUsesExitFunction verifies that main calls exitFunction with
// the code returned by run.
func TestMainUsesExitFunction(t *testing.T) {
	oldExit := exitFunction
	defer func() { exitFunction = oldExit }()

	called := false
	var gotCode int
	exitFunction = func(code int) {
		called = true
		gotCode = code
	}

	arguments := os.Args
	defer func() { os.Args = arguments }()
	os.Args = []string{"heft", "--version"}

	main()

	if !called {
		t.Fatalf("expected exitFunction to be called")
	}
	if gotCode != 0 {
		t.Fatalf("expected exit code 0, got %d", gotCode)
	}
}

// TestRunInvokesCLIExecuteForNormalArgs ensures that when no version
// flags are present, run delegates to cli.Execute and returns 0.
func TestRunInvokesCLIExecuteForNormalArgs(t *testing.T) {
	// There is no hook to observe cli.Execute here without changing
	// production code, but this test exercises the branch where no
	// version-related arguments are present and ensures run returns 0.
	code := run([]string{"scan", "my-chart"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}
