package main

import (
	"errors"
	"testing"
)

// TestRunReturnsErrorWhenRepositoryRootFails exercises the early error
// path where repositoryRootFunc returns an error.
func TestRunReturnsErrorWhenRepositoryRootFails(t *testing.T) {
	oldRepoRoot := repositoryRootFunc
	defer func() { repositoryRootFunc = oldRepoRoot }()

	repositoryRootFunc = func() (string, error) {
		return "", errors.New("boom")
	}

	if err := run(1, "high", "stars", false); err == nil {
		t.Fatalf("expected error from run when repositoryRoot fails")
	}
}

// TestRunReturnsErrorWhenEnsureHeftBinaryFails covers the error path
// where ensureHeftBinaryFunc returns an error.
func TestRunReturnsErrorWhenEnsureHeftBinaryFails(t *testing.T) {
	oldRepoRoot := repositoryRootFunc
	oldEnsure := ensureHeftBinaryFunc
	defer func() {
		repositoryRootFunc = oldRepoRoot
		ensureHeftBinaryFunc = oldEnsure
	}()

	repositoryRootFunc = func() (string, error) {
		return "/tmp/repo", nil
	}

	ensureHeftBinaryFunc = func(repoRoot string) (string, error) {
		return "", errors.New("no heft")
	}

	if err := run(1, "high", "stars", false); err == nil {
		t.Fatalf("expected error from run when ensureHeftBinary fails")
	}
}

// TestRunReturnsErrorWhenFetchArtifactHubChartsFails covers the error
// propagation when fetchArtifactHubChartsFunc returns an error.
func TestRunReturnsErrorWhenFetchArtifactHubChartsFails(t *testing.T) {
	oldRepoRoot := repositoryRootFunc
	oldEnsure := ensureHeftBinaryFunc
	oldFetch := fetchArtifactHubChartsFunc
	defer func() {
		repositoryRootFunc = oldRepoRoot
		ensureHeftBinaryFunc = oldEnsure
		fetchArtifactHubChartsFunc = oldFetch
	}()

	repositoryRootFunc = func() (string, error) {
		return "/tmp/repo", nil
	}

	ensureHeftBinaryFunc = func(repoRoot string) (string, error) {
		return "/bin/heft", nil
	}

	fetchArtifactHubChartsFunc = func(limit, offset int, sort string) ([]artifactHubChart, error) {
		return nil, errors.New("fetch failed")
	}

	if err := run(1, "high", "stars", false); err == nil {
		t.Fatalf("expected error from run when fetchArtifactHubCharts fails")
	}
}
