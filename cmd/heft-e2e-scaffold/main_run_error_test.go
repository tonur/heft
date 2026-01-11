package main

import "testing"

// TestRunWithZeroMaxChartsAndNoCharts ensures that when maxCharts is
// zero and Artifact Hub returns no charts, run completes without error.
// This exercises the loop condition and empty-page handling.
func TestRunWithZeroMaxChartsAndNoCharts(t *testing.T) {
	if err := run(0, "high", "stars", false); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}
