package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestRunStopsWhenNoChartsEnsures the len(charts) == 0 branch is covered.
func TestRunStopsWhenNoCharts(t *testing.T) {
	repository := t.TempDir()
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example.com/heft-test"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	// Point ensureHeftBinary at a fake path so it does not try to build.
	// We rely on HEFT_BINARY to satisfy ensureHeftBinary without running it.
	fakeHeft := filepath.Join(repository, "heft")
	if err := os.WriteFile(fakeHeft, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile fake heft: %v", err)
	}
	t.Setenv("HEFT_BINARY", fakeHeft)

	// Artifact Hub server returning no charts.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(artifactHubSearchResponse{Charts: nil})
	}))
	defer testServer.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = testServer.URL
	defer func() { artifactHubBaseURL = oldBase }()

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(oldWorkingDirectory)
	if err := os.Chdir(repository); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	if err := run(1, "high", "stars", false); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
