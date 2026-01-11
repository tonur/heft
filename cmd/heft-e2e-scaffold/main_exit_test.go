package main

import (
	"os"
	"testing"
)

// TestMainUsesExitFunction verifies that main calls exitFunction(1)
// when run returns an error.
func TestMainUsesExitFunction(t *testing.T) {
	oldExit := exitFunction
	defer func() { exitFunction = oldExit }()

	called := false
	var gotCode int
	exitFunction = func(code int) {
		called = true
		gotCode = code
	}

	// Arrange for run to fail by pointing repositoryRoot at a directory
	// with no go.mod so run will return an error.
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(oldWD)

	// Use a temporary directory without go.mod.
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	main()

	if !called {
		t.Fatalf("expected exitFunction to be called")
	}
	if gotCode != 1 {
		t.Fatalf("expected exit code 1, got %d", gotCode)
	}
}
