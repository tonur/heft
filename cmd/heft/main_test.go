package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestRunVersionFlag ensures that the run helper handles the
// --version flag and returns a zero exit code without invoking exitFunc.
func TestRunVersionFlag(t *testing.T) {
	code := run([]string{"--version"})
	if code != 0 {
		t.Fatalf("expected exit code 0 from run, got %d", code)
	}
}

// TestMainVersionFlag ensures that the --version flag is handled by
// the main entrypoint and prints a non-empty version string.
func TestMainVersionFlag(t *testing.T) {
	// Run `go run` on the heft command from the repo root.
	cmd := exec.Command("go", "run", "./cmd/heft", "--version")
	cmd.Dir = "../.."

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/heft --version failed: %v\noutput: %s", err, string(out))
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		t.Fatalf("expected non-empty version output, got %q", version)
	}
}

// TestMainUsesExitFunc verifies that main calls exitFunc with
// the code returned by run.
func TestMainUsesExitFunc(t *testing.T) {
	oldExit := exitFunc
	defer func() { exitFunc = oldExit }()

	called := false
	var gotCode int
	exitFunc = func(code int) {
		called = true
		gotCode = code
	}

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"heft", "--version"}

	main()

	if !called {
		t.Fatalf("expected exitFunc to be called")
	}
	if gotCode != 0 {
		t.Fatalf("expected exit code 0, got %d", gotCode)
	}
}
