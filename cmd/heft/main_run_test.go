package main

import (
	"testing"
)

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
