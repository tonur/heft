package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchArtifactHubChartsHTTPError ensures non-200 responses are surfaced
// as errors with some context.
func TestFetchArtifactHubChartsHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("nope"))
	}))
	defer ts.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = ts.URL
	defer func() { artifactHubBaseURL = oldBase }()

	_, err := fetchArtifactHubCharts(1, 0, "stars")
	if err == nil {
		t.Fatalf("expected error from non-200 response, got nil")
	}
}

// TestResolveFromHelmIndexErrorCases covers several error branches from
// resolveFromHelmIndex: empty chart name, missing chart, and non-200 index.
func TestResolveFromHelmIndexErrorCases(t *testing.T) {
	if _, err := resolveFromHelmIndex("https://example.com/repo", "", ""); err == nil {
		t.Fatalf("expected error for empty chart name, got nil")
	}

	// Non-200 status from index.yaml.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("missing"))
	}))
	defer ts.Close()

	if _, err := resolveFromHelmIndex(ts.URL, "chart", ""); err == nil {
		t.Fatalf("expected error for non-200 index response, got nil")
	}

	// Valid YAML but missing chart entry.
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": map[string]any{}})
	}))
	defer good.Close()

	if _, err := resolveFromHelmIndex(good.URL, "missing", ""); err == nil {
		t.Fatalf("expected error for missing chart entry, got nil")
	}
}

// TestResolveChartURLOCIFallback ensures resolveChartURL exercises the OCI
// heuristic branch when repository kind is non-helm but URL suggests an OCI
// repository.
func TestResolveChartURLOCIFallback(t *testing.T) {
	chart := artifactHubChart{}
	chart.Name = "mychart"
	chart.Repository.Kind = 1
	chart.Repository.URL = "https://ghcr.io/org/charts"

	resolved, err := resolveChartURL(chart)
	if err != nil {
		t.Fatalf("resolveChartURL unexpected error: %v", err)
	}
	if resolved == "" {
		t.Fatalf("expected non-empty resolved URL for OCI fallback")
	}
}
