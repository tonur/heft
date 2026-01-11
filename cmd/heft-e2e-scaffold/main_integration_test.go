package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRepositoryRootFindsGoMod ensures repositoryRoot walks up to find go.mod.
func TestRepositoryRootFindsGoMod(t *testing.T) {
	repoDir, err := os.MkdirTemp("", "heft-repo-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(repoDir)

	goModPath := filepath.Join(repoDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module example.com/heft-test"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	subDir := filepath.Join(repoDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("MkdirAll subdir: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(oldWD)

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	root, err := repositoryRoot()
	if err != nil {
		t.Fatalf("repositoryRoot returned error: %v", err)
	}
	if root != repoDir {
		t.Fatalf("repositoryRoot = %q, want %q", root, repoDir)
	}
}

// TestResolveChartURLErrors exercises error branches where repository URL
// is missing or the repository kind is unsupported.
func TestResolveChartURLErrors(t *testing.T) {
	// Missing repository URL
	chart := artifactHubChart{}
	_, err := resolveChartURL(chart)
	if err == nil {
		t.Fatalf("expected error for missing repository URL, got nil")
	}

	// Unsupported kind and non-OCI URL
	chart.Repository.Kind = 1
	chart.Repository.URL = "https://example.com/unsupported"
	_, err = resolveChartURL(chart)
	if err == nil {
		t.Fatalf("expected error for unsupported repository kind, got nil")
	}
}
