package scan

import "testing"

func TestRunIsDeprecated(t *testing.T) {
	if err := Run(); err == nil {
		t.Fatalf("expected error from Run, got nil")
	}
}
