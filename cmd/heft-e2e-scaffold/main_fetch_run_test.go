package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFetchArtifactHubChartsUsesBaseURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/packages/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(artifactHubSearchResponse{
			Charts: []artifactHubChart{{Name: "test", NormalizedName: "test", Version: "1.0.0"}},
		})
	}))
	defer ts.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = ts.URL
	defer func() { artifactHubBaseURL = oldBase }()

	charts, err := fetchArtifactHubCharts(10, 0, "stars")
	if err != nil {
		t.Fatalf("fetchArtifactHubCharts error: %v", err)
	}
	if len(charts) != 1 || charts[0].Name != "test" {
		t.Fatalf("unexpected charts: %+v", charts)
	}
}

func TestRunScaffoldsFromFakeArtifactHub(t *testing.T) {
	repo := t.TempDir()
	// Minimal go.mod to satisfy repositoryRoot when run under this directory.
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/heft-test"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	// Prepare fake heft binary that prints deterministic YAML.
	tdir := t.TempDir()
	fakeHeft := filepath.Join(tdir, "heft")
	fakeSrc := filepath.Join(tdir, "main.go")
	if err := os.WriteFile(fakeSrc, []byte(`package main
import "fmt"
func main() {
	fmt.Println("images:\n- name: alpine\n  confidence: high\n  source: rendered-manifest")
}
`), 0o644); err != nil {
		t.Fatalf("write fake heft source: %v", err)
	}
	buildCmd := exec.Command("go", "build", "-o", fakeHeft, fakeSrc)
	buildCmd.Env = os.Environ()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake heft: %v\n%s", err, string(out))
	}

	// Use environment variable to point ensureHeftBinary at our fake heft.
	t.Setenv("HEFT_BINARY", fakeHeft)

	// Fake Artifact Hub server returning a single chart.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(artifactHubSearchResponse{
			Charts: []artifactHubChart{{
				Name:           "testchart",
				NormalizedName: "testchart",
				Version:        "1.0.0",
				ContentURL:     "https://example.com/testchart-1.0.0.tgz",
			}},
		})
	}))
	defer ts.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = ts.URL
	defer func() { artifactHubBaseURL = oldBase }()

	// Run from inside the temporary repo so repositoryRoot sees our go.mod.
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(oldWD)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	if err := run(1, "high", "stars", false); err != nil {
		t.Fatalf("run error: %v", err)
	}

	chartsRoot := filepath.Join(repo, "internal", "system", "testdata", "charts")
	if _, err := os.Stat(chartsRoot); err != nil {
		t.Fatalf("expected charts root at %s: %v", chartsRoot, err)
	}
}
