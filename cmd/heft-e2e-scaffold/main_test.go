package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeImageName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{"empty", "", ""},
		{"alreadyDocker", "docker.io/library/nginx:1.2.3", "docker.io/library/nginx:1.2.3"},
		{"ghcrUnchanged", "ghcr.io/org/app:1.0.0", "ghcr.io/org/app:1.0.0"},
		{"bareWithTag", "kong:3.9", "docker.io/library/kong:3.9"},
		{"bareNoTag", "alpine", "docker.io/library/alpine:latest"},
		{"userRepoWithTag", "tonur/i-am-root:1.0", "docker.io/tonur/i-am-root:1.0"},
		{"userRepoNoTag", "tonur/i-am-root", "docker.io/tonur/i-am-root:latest"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeImageName(tt.in)
			if got != tt.out {
				t.Fatalf("normalizeImageName(%q) = %q, want %q", tt.in, got, tt.out)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "foo", "bar"); got != "foo" {
		t.Fatalf("firstNonEmpty returned %q, want %q", got, "foo")
	}
	if got := firstNonEmpty("", "   ", "bar"); got != "bar" {
		t.Fatalf("firstNonEmpty returned %q, want %q", got, "bar")
	}
	if got := firstNonEmpty(); got != "" {
		t.Fatalf("firstNonEmpty with no args = %q, want empty", got)
	}
}

func TestOCIURLFromRepo(t *testing.T) {
	cases := []struct {
		name       string
		repository string
		chart      string
		want       string
		wantErr    bool
	}{
		{"emptyChart", "https://ghcr.io/org/charts", "", "", true},
		{"httpsRepo", "https://ghcr.io/org/charts", "mychart", "oci://ghcr.io/org/charts/mychart", false},
		{"ociRepo", "oci://ghcr.io/org/charts", "mychart", "oci://ghcr.io/org/charts/mychart", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ociURLFromRepository(testCase.repository, testCase.chart)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil and %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("ociURLFromRepo(%q,%q) = %q, want %q", testCase.repository, testCase.chart, got, testCase.want)
			}
		})
	}
}

func TestResolveFromHelmIndexStablePreferred(t *testing.T) {
	indexYAML := `entries:
  mychart:
    - version: "1.0.0"
      urls:
        - "mychart-1.0.0.tgz"
    - version: "1.1.0-beta.1"
      urls:
        - "mychart-1.1.0-beta.1.tgz"
`

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		_, _ = w.Write([]byte(indexYAML))
	}))
	defer testServer.Close()

	repositoryURL := testServer.URL

	url, err := resolveFromHelmIndex(repositoryURL, "mychart", "")
	if err != nil {
		t.Fatalf("resolveFromHelmIndex unexpected error: %v", err)
	}
	if url != testServer.URL+"/mychart-1.0.0.tgz" {
		t.Fatalf("resolveFromHelmIndex = %q, want %q", url, testServer.URL+"/mychart-1.0.0.tgz")
	}
}

func TestResolveFromHelmIndexExactVersion(t *testing.T) {
	indexYAML := `entries:
  mychart:
    - version: "1.0.0"
      urls:
        - "mychart-1.0.0.tgz"
    - version: "1.1.0-beta.1"
      urls:
        - "mychart-1.1.0-beta.1.tgz"
`

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		_, _ = w.Write([]byte(indexYAML))
	}))
	defer testServer.Close()

	repoURL := testServer.URL

	got, err := resolveFromHelmIndex(repoURL, "mychart", "1.1.0-beta.1")
	if err != nil {
		t.Fatalf("resolveFromHelmIndex unexpected error: %v", err)
	}
	if got != testServer.URL+"/mychart-1.1.0-beta.1.tgz" {
		t.Fatalf("resolveFromHelmIndex = %q, want %q", got, testServer.URL+"/mychart-1.1.0-beta.1.tgz")
	}
}

func TestResolveFromHelmIndexPreReleaseOnly(t *testing.T) {
	indexYAML := `entries:
  mychart:
    - version: "1.0.0-beta.1"
      urls:
        - "mychart-1.0.0-beta.1.tgz"
    - version: "1.0.0-beta.2"
      urls:
        - "mychart-1.0.0-beta.2.tgz"
`

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		_, _ = w.Write([]byte(indexYAML))
	}))
	defer testServer.Close()

	repoURL := testServer.URL

	got, err := resolveFromHelmIndex(repoURL, "mychart", "")
	if err != nil {
		t.Fatalf("resolveFromHelmIndex unexpected error: %v", err)
	}
	if got != testServer.URL+"/mychart-1.0.0-beta.1.tgz" {
		t.Fatalf("resolveFromHelmIndex = %q, want %q", got, testServer.URL+"/mychart-1.0.0-beta.1.tgz")
	}
}

func TestResolveChartURLPrefersContentURL(t *testing.T) {
	chart := artifactHubChart{
		Name:       "testchart",
		ContentURL: "https://example.com/testchart-1.0.0.tgz",
	}
	got, err := resolveChartURL(chart)
	if err != nil {
		t.Fatalf("resolveChartURL unexpected error: %v", err)
	}
	if got != chart.ContentURL {
		t.Fatalf("resolveChartURL = %q, want %q", got, chart.ContentURL)
	}
}
