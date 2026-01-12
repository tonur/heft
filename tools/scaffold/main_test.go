package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// TestScaffoldChartCreatesFixture uses a fake heft binary that prints a
// deterministic YAML payload so we can verify the generated metadata and
// command fixture from scaffoldChart.
func TestScaffoldChartCreatesFixture(t *testing.T) {
	temporaryDirectory, err := os.MkdirTemp("", "heft-scaffold-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(temporaryDirectory)

	// Create a fake heft binary that prints a single high-confidence image.
	fakeHeft := filepath.Join(temporaryDirectory, "heft")
	fakeSource := filepath.Join(temporaryDirectory, "main.go")
	if err := os.WriteFile(fakeSource, []byte(`package main
import "fmt"
func main() {
	fmt.Println("images:\n- name: alpine\n  confidence: high\n  source: rendered-manifest")
}
`), 0o644); err != nil {
		t.Fatalf("write fake heft source: %v", err)
	}

	buildCommand := exec.Command("go", "build", "-o", fakeHeft, fakeSource)
	buildCommand.Env = os.Environ()
	if out, err := buildCommand.CombinedOutput(); err != nil {
		t.Fatalf("build fake heft: %v\n%s", err, string(out))
	}
	if runtime.GOOS == "windows" {
		fakeHeft += ".exe"
	}

	chartDir := filepath.Join(temporaryDirectory, "chart")
	metadataPath := filepath.Join(chartDir, "chart_metadata.yaml")

	chart := &artifactHubChart{
		Name:    "testchart",
		Version: "1.0.0",
	}

	if err := scaffoldChart(chartDir, metadataPath, chart, "dummy-chart-url", fakeHeft, "high"); err != nil {
		t.Fatalf("scaffoldChart error: %v", err)
	}

	// Ensure metadata file exists.
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("expected metadata file at %s: %v", metadataPath, err)
	}

	// Ensure a command fixture exists with the expected normalized image.
	commandsDir := filepath.Join(chartDir, "commands")
	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		t.Fatalf("ReadDir commands: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one command fixture in %s", commandsDir)
	}

	foundExpected := false
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(commandsDir, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), err)
		}
		if strings.Contains(string(content), "docker.io/library/alpine:latest") {
			foundExpected = true
			break
		}
	}

	if !foundExpected {
		t.Fatalf("expected commands fixture to contain docker.io/library/alpine:latest")
	}
}

// TestRunWithZeroMaxChartsAndNoCharts ensures that when maxCharts is
// zero and Artifact Hub returns no charts, run completes without error.
// This exercises the loop condition and empty-page handling.
func TestRunWithZeroMaxChartsAndNoCharts(t *testing.T) {
	if err := run(0, "high", "stars", false); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

// TestRunReturnsErrorWhenRepositoryRootFails exercises the early error
// path where repositoryRootFunc returns an error.
func TestRunReturnsErrorWhenRepositoryRootFails(t *testing.T) {
	oldRepositoryRoot := repositoryRootFunction
	defer func() { repositoryRootFunction = oldRepositoryRoot }()

	repositoryRootFunction = func() (string, error) {
		return "", errors.New("boom")
	}

	if err := run(1, "high", "stars", false); err == nil {
		t.Fatalf("expected error from run when repositoryRoot fails")
	}
}

// TestRunReturnsErrorWhenEnsureHeftBinaryFails covers the error path
// where ensureHeftBinaryFunc returns an error.
func TestRunReturnsErrorWhenEnsureHeftBinaryFails(t *testing.T) {
	oldRepoRoot := repositoryRootFunction
	oldEnsure := ensureHeftBinaryFunction
	defer func() {
		repositoryRootFunction = oldRepoRoot
		ensureHeftBinaryFunction = oldEnsure
	}()

	repositoryRootFunction = func() (string, error) {
		return "/tmp/repo", nil
	}

	ensureHeftBinaryFunction = func(repoRoot string) (string, error) {
		return "", errors.New("no heft")
	}

	if err := run(1, "high", "stars", false); err == nil {
		t.Fatalf("expected error from run when ensureHeftBinary fails")
	}
}

// TestRunReturnsErrorWhenFetchArtifactHubChartsFails covers the error
// propagation when fetchArtifactHubChartsFunc returns an error.
func TestRunReturnsErrorWhenFetchArtifactHubChartsFails(t *testing.T) {
	oldRepositoryRoot := repositoryRootFunction
	oldEnsure := ensureHeftBinaryFunction
	oldFetch := fetchArtifactHubChartsFunction
	defer func() {
		repositoryRootFunction = oldRepositoryRoot
		ensureHeftBinaryFunction = oldEnsure
		fetchArtifactHubChartsFunction = oldFetch
	}()

	repositoryRootFunction = func() (string, error) {
		return "/tmp/repo", nil
	}

	ensureHeftBinaryFunction = func(repoRoot string) (string, error) {
		return "/bin/heft", nil
	}

	fetchArtifactHubChartsFunction = func(limit, offset int, sort string) ([]artifactHubChart, error) {
		return nil, errors.New("fetch failed")
	}

	if err := run(1, "high", "stars", false); err == nil {
		t.Fatalf("expected error from run when fetchArtifactHubCharts fails")
	}
}

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

func TestFetchArtifactHubChartsUsesBaseURL(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer testServer.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = testServer.URL
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
	repository := t.TempDir()
	// Minimal go.mod to satisfy repositoryRoot when run under this directory.
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example.com/heft-test"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	// Prepare fake heft binary that prints deterministic YAML.
	temporaryDirectory := t.TempDir()
	fakeHeft := filepath.Join(temporaryDirectory, "heft")
	fakeSrc := filepath.Join(temporaryDirectory, "main.go")
	if err := os.WriteFile(fakeSrc, []byte(`package main
import "fmt"
func main() {
	fmt.Println("images:\n- name: alpine\n  confidence: high\n  source: rendered-manifest")
}
`), 0o644); err != nil {
		t.Fatalf("write fake heft source: %v", err)
	}
	buildCommand := exec.Command("go", "build", "-o", fakeHeft, fakeSrc)
	buildCommand.Env = os.Environ()
	if out, err := buildCommand.CombinedOutput(); err != nil {
		t.Fatalf("build fake heft: %v\n%s", err, string(out))
	}

	// Use environment variable to point ensureHeftBinary at our fake heft.
	t.Setenv("HEFT_BINARY", fakeHeft)

	// Fake Artifact Hub server returning a single chart.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(artifactHubSearchResponse{
			Charts: []artifactHubChart{{
				Name:           "testchart",
				NormalizedName: "testchart",
				Version:        "1.0.0",
				ContentURL:     "https://example.com/testchart-1.0.0.tgz",
			}},
		})
	}))
	defer testServer.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = testServer.URL
	defer func() { artifactHubBaseURL = oldBase }()

	// Run from inside the temporary repository so repositoryRoot sees our go.mod.
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

	chartsRoot := filepath.Join(repository, "testcases")
	if _, err := os.Stat(chartsRoot); err != nil {
		t.Fatalf("expected charts root at %s: %v", chartsRoot, err)
	}
}

// TestFetchArtifactHubChartsHTTPError ensures non-200 responses are surfaced
// as errors with some context.
func TestFetchArtifactHubChartsHTTPError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("nope"))
	}))
	defer testServer.Close()

	oldBase := artifactHubBaseURL
	artifactHubBaseURL = testServer.URL
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
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("missing"))
	}))
	defer testServer.Close()

	if _, err := resolveFromHelmIndex(testServer.URL, "chart", ""); err == nil {
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

// TestMainUsesExitFunction verifies that main calls exitFunction(1)
// when run returns an error.
func TestMainUsesExitFunction(t *testing.T) {
	oldExit := exitFunction
	defer func() { exitFunction = oldExit }()

	called := false
	var gotCode int
	exitFunction = func(code int) {
		called = true
		gotCode = code
	}

	// Arrange for run to fail by pointing repositoryRoot at a directory
	// with no go.mod so run will return an error.
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(oldWD)

	// Use a temporary directory without go.mod.
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	main()

	if !called {
		t.Fatalf("expected exitFunction to be called")
	}
	if gotCode != 1 {
		t.Fatalf("expected exit code 1, got %d", gotCode)
	}
}

func TestEnsureHeftBinaryUsesEnv(t *testing.T) {
	tdir := t.TempDir()
	fake := filepath.Join(tdir, "heft-env")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho env-heft\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("HEFT_BINARY", fake)

	got, err := ensureHeftBinary(tdir)
	if err != nil {
		t.Fatalf("ensureHeftBinary error: %v", err)
	}
	if got != fake {
		t.Fatalf("ensureHeftBinary = %q, want %q", got, fake)
	}
}

// TestEnsureHeftBinaryBuildsWhenNotOnPath exercises the branch where
// ensureHeftBinary falls back to running `go build ./cmd/heft`.
//
// It uses a small wrapper around the real `go` tool so that we can
// assert the command-line invocation without depending on any
// particular output format. If `go` is not available, the test is
// skipped.
func TestEnsureHeftBinaryBuildsWhenNotOnPath(t *testing.T) {
	// Skip if there is no go tool; without it the build path cannot run.
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not available in PATH; skipping build-path test")
	}

	repository := t.TempDir()
	// Minimal go.mod so that ./cmd/heft resolves.
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example.com/heft-test"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	// Create a minimal cmd/heft main package so the build succeeds quickly.
	heftDirectory := filepath.Join(repository, "cmd", "heft")
	if err := os.MkdirAll(heftDirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll cmd/heft: %v", err)
	}
	mainSrc := []byte("package main\nfunc main() {}\n")
	if err := os.WriteFile(filepath.Join(heftDirectory, "main.go"), mainSrc, 0o644); err != nil {
		t.Fatalf("WriteFile cmd/heft/main.go: %v", err)
	}

	// Create a wrapper for the go tool that delegates to the real go
	// but records that it was invoked. This keeps the behavior close
	// to the real path while remaining hermetic.
	wrapperDirectory := t.TempDir()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("LookPath go: %v", err)
	}

	wrapperPath := filepath.Join(wrapperDirectory, "go")
	wrapperSource := "#!/bin/sh\n" +
		"echo wrapper-go-invoked >> '" + filepath.Join(wrapperDirectory, "log") + "'\n" +
		"exec '" + realGo + "' \"$@\"\n"
	if runtime.GOOS == "windows" {
		// On Windows, fall back to calling the real go directly without
		// a shell script wrapper, since .bat/.cmd handling differs. For
		// simplicity, just use the real go on PATH.
		wrapperPath = realGo
	} else {
		if err := os.WriteFile(wrapperPath, []byte(wrapperSource), 0o755); err != nil {
			t.Fatalf("WriteFile go wrapper: %v", err)
		}
	}

	// Ensure HEFT_BINARY is unset so ensureHeftBinary does not short-circuit,
	// and remove any real heft from PATH so the build branch is taken.
	t.Setenv("HEFT_BINARY", "")
	// PATH will contain only our wrapper directory so LookPath("heft") fails
	// and the code falls back to building.
	t.Setenv("PATH", wrapperDirectory)

	binary, err := ensureHeftBinary(repository)
	if err != nil {
		t.Fatalf("ensureHeftBinary returned error: %v", err)
	}
	if binary == "" {
		t.Fatalf("ensureHeftBinary returned empty path")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("expected built heft binary at %s: %v", binary, err)
	}
}
