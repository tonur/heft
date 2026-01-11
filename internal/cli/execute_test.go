package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

// TestExecuteUsesExitFunctionOnError verifies that Execute calls exitFunc
// with code 1 when the underlying command returns an error.
func TestExecuteUsesExitFunctionOnError(t *testing.T) {
	oldExit := exitFunction
	oldExecute := executeCommand
	defer func() {
		exitFunction = oldExit
		executeCommand = oldExecute
	}()

	// Make executeCommand always return an error.
	executeCommand = func(command *cobra.Command) error {
		return errors.New("boom")
	}

	called := false
	var gotCode int
	exitFunction = func(code int) {
		called = true
		gotCode = code
	}

	Execute()

	if !called {
		t.Fatalf("expected exitFunc to be called on error")
	}
	if gotCode != 1 {
		t.Fatalf("expected exit code 1, got %d", gotCode)
	}
}
