package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchArtifactHubChartsHandlesNon200Status(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad"))
	}))
	defer testServer.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = testServer.URL
	defer func() { artifactHubBaseURL = oldBase }()

	if _, err := fetchArtifactHubCharts(10, 0, "stars"); err == nil {
		t.Fatalf("expected error for non-200 status, got nil")
	}
}

func TestFetchArtifactHubChartsHandlesInvalidJSON(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json"))
	}))
	defer testServer.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = testServer.URL
	defer func() { artifactHubBaseURL = oldBase }()

	if _, err := fetchArtifactHubCharts(10, 0, "stars"); err == nil {
		t.Fatalf("expected error for invalid JSON, got nil")
	}
}
