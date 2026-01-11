package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJoinHelmURLInvalidRepository(t *testing.T) {
	_, err := joinHelmURL("://bad-url", "chart.tgz")
	if err == nil {
		t.Fatalf("expected error for invalid repository URL")
	}
}

func TestJoinHelmURLInvalidChart(t *testing.T) {
	_, err := joinHelmURL("https://example.com", ":://bad-chart")
	if err == nil {
		t.Fatalf("expected error for invalid chart URL")
	}
}

func TestResolveFromHelmIndexChartNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "entries:\n  other:\n    - version: 1.0.0\n      urls:\n        - other-1.0.0.tgz")
	}))
	defer server.Close()

	_, err := resolveFromHelmIndex(server.URL, "missing", "")
	if err == nil || err.Error() != "chart missing not found in index" {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestResolveFromHelmIndexNoURLsForVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "entries:\n  mychart:\n    - version: 1.2.3\n      urls: []")
	}))
	defer server.Close()

	_, err := resolveFromHelmIndex(server.URL, "mychart", "1.2.3")
	if err == nil || err.Error() != "no URLs for chart mychart version 1.2.3" {
		t.Fatalf("expected no URLs for version error, got %v", err)
	}
}

func TestResolveFromHelmIndexNoURLsForBestVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "entries:\n  mychart:\n    - version: 1.2.3\n      urls: []")
	}))
	defer server.Close()

	_, err := resolveFromHelmIndex(server.URL, "mychart", "")
	if err == nil || err.Error() != "no URLs for chart mychart best version" {
		t.Fatalf("expected no URLs for best version error, got %v", err)
	}
}

func TestResolveFromHelmIndexEmptyChartName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "entries: {}")
	}))
	defer server.Close()

	_, err := resolveFromHelmIndex(server.URL, "", "")
	if err == nil || err.Error() != "chart name is empty" {
		t.Fatalf("expected chart name is empty error, got %v", err)
	}
}

func TestLoadHelmIndexNetworkError(t *testing.T) {
	// Use an invalid URL to force http.Get error.
	_, err := loadHelmIndex("http://[::1")
	if err == nil {
		t.Fatalf("expected error from loadHelmIndex with invalid URL")
	}
}

func TestLoadHelmIndexNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "nope")
	}))
	defer server.Close()

	_, err := loadHelmIndex(server.URL)
	if err == nil || err.Error() != "index.yaml status 500: nope" {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestLoadHelmIndexInvalidYAML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not: [yaml")
	}))
	defer server.Close()

	_, err := loadHelmIndex(server.URL)
	if err == nil {
		t.Fatalf("expected parse error from invalid YAML")
	}
}
