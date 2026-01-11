package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type artifactHubSearchResponse struct {
	Charts []artifactHubChart `json:"packages"`
}

type artifactHubChart struct {
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name"`
	Version        string `json:"version"`
	ContentURL     string `json:"content_url"`
	Repository     struct {
		Kind int    `json:"kind"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"repository"`
}

type scanImage struct {
	Name       string `yaml:"name"`
	Confidence string `yaml:"confidence"`
	Source     string `yaml:"source"`
}

type scanOutput struct {
	Images []scanImage `yaml:"images"`
}

type expectedImage struct {
	Image      string `yaml:"image"`
	Confidence string `yaml:"confidence"`
	Source     string `yaml:"source"`
}

type chartMetadata struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Version string `yaml:"version,omitempty"`
	Source  string `yaml:"source,omitempty"`
}

type commandFixture struct {
	Name           string          `yaml:"name"`
	Arguments      []string        `yaml:"arguments"`
	ExpectedImages []expectedImage `yaml:"expectedImages"`
}

func main() {
	maxCharts := flag.Int("max-charts", 10, "maximum number of new charts to scaffold")
	minConfidence := flag.String("min-confidence", "high", "minimum confidence passed to heft scan")
	artifactHubSort := flag.String("sort", "stars", "sort field for Artifact Hub search (e.g. stars, score)")
	flag.Parse()

	if err := run(*maxCharts, *minConfidence, *artifactHubSort); err != nil {
		fmt.Fprintf(os.Stderr, "heft-e2e-scaffold error: %v\n", err)
		os.Exit(1)
	}
}

func run(maxCharts int, minConfidence, sort string) error {
	repositoryRoot, err := repositoryRoot()
	if err != nil {
		return err
	}

	heftPath, err := ensureHeftBinary(repositoryRoot)
	if err != nil {
		return fmt.Errorf("ensure heft binary: %w", err)
	}

	fmt.Printf("Using heft binary at %s\n", heftPath)

	chartsRoot := filepath.Join(repositoryRoot, "internal", "system", "testdata", "charts")
	if err := os.MkdirAll(chartsRoot, 0o755); err != nil {
		return fmt.Errorf("create chartsRoot: %w", err)
	}

	newCharts := 0
	offset := 0
	pageLimit := 60

	for newCharts < maxCharts {
		charts, err := fetchArtifactHubCharts(pageLimit, offset, sort)
		if err != nil {
			return fmt.Errorf("fetch Artifact Hub charts: %w", err)
		}
		if len(charts) == 0 {
			break
		}

		for _, chart := range charts {
			if newCharts >= maxCharts {
				break
			}

			name := chart.NormalizedName
			if name == "" {
				name = chart.Name
			}
			if name == "" {
				continue
			}

			chartDir := filepath.Join(chartsRoot, name)
			metadataPath := filepath.Join(chartDir, "chart_metadata.yaml")
			if _, err := os.Stat(metadataPath); err == nil {
				// already scaffolded
				continue
			}

			chartURL, err := resolveChartURL(chart)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Skipping %s: cannot resolve chart ref: %v\n", name, err)
				continue
			}

			if err := scaffoldChart(chartDir, metadataPath, &chart, chartURL, heftPath, minConfidence); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to scaffold chart %s: %v\n", name, err)
				continue
			}
			newCharts++
		}

		offset += pageLimit
		// Be polite to Artifact Hub.
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("Scaffolded %d new chart test cases\n", newCharts)
	return nil
}

func repositoryRoot() (string, error) {
	// Assume this binary is run from repo root or below.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up until we find go.mod as a crude repo root marker.
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod from %s", cwd)
		}
		dir = parent
	}
}

func ensureHeftBinary(repoRoot string) (string, error) {
	// If HEFT_BINARY is set, use that.
	if p := os.Getenv("HEFT_BINARY"); p != "" {
		return p, nil
	}

	// If "heft" is on PATH, prefer that to avoid rebuilding every time.
	if p, err := exec.LookPath("heft"); err == nil {
		return p, nil
	}

	// Otherwise, build a fresh heft in a temp dir.
	tmpDir, err := os.MkdirTemp("", "heft-e2e-scaffold-")
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(tmpDir, "heft")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/heft")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("building heft failed: %v\n%s", err, string(out))
	}

	return binPath, nil
}

func fetchArtifactHubCharts(limit, offset int, sort string) ([]artifactHubChart, error) {
	url := fmt.Sprintf("https://artifacthub.io/api/v1/packages/search?kind=0&limit=%d&offset=%d&sort=%s", limit, offset, sort)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("artifacthub search: status %d: %s", response.StatusCode, string(body))
	}

	var searchResponse artifactHubSearchResponse
	if err := json.NewDecoder(response.Body).Decode(&searchResponse); err != nil {
		return nil, err
	}

	return searchResponse.Charts, nil
}

func scaffoldChart(chartDir, metadataPath string, chart *artifactHubChart, chartURL, heftPath, minConfidence string) error {
	if err := os.MkdirAll(filepath.Join(chartDir, "commands"), 0o755); err != nil {
		return fmt.Errorf("create chart dir: %w", err)
	}

	md := chartMetadata{
		Name:    firstNonEmpty(chart.NormalizedName, chart.Name),
		URL:     chartURL,
		Version: chart.Version,
		Source:  "artifacthub",
	}

	mdBytes, err := yaml.Marshal(&md)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, mdBytes, 0o644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	// Run heft scan to populate expected images.
	images, err := runHeftScanForImages(heftPath, chartURL, minConfidence)
	if err != nil {
		return fmt.Errorf("heft scan: %w", err)
	}

	// Map to expectedImages, optionally filter to high confidence only.
	var expected []expectedImage
	for _, img := range images {
		if img.Confidence != "high" {
			continue
		}
		expected = append(expected, expectedImage{
			Image:      normalizeImageName(img.Name),
			Confidence: img.Confidence,
			Source:     img.Source,
		})
	}

	if len(expected) == 0 {
		// Fall back to including all images so the test isn't trivially empty.
		for _, img := range images {
			expected = append(expected, expectedImage{
				Image:      normalizeImageName(img.Name),
				Confidence: img.Confidence,
				Source:     img.Source,
			})
		}
	}

	fixture := commandFixture{
		Name:           "min-confidence-" + minConfidence,
		Arguments:      []string{"scan", "${CHART_URL}", "--min-confidence=" + minConfidence},
		ExpectedImages: expected,
	}

	fixtureBytes, err := yaml.Marshal(&fixture)
	if err != nil {
		return fmt.Errorf("marshal command fixture: %w", err)
	}

	fixturePath := filepath.Join(chartDir, "commands", fixture.Name+".yaml")
	if err := os.WriteFile(fixturePath, fixtureBytes, 0o644); err != nil {
		return fmt.Errorf("write command fixture: %w", err)
	}

	fmt.Printf("Scaffolded chart %s (%s) -> %s\n", md.Name, md.Version, chartDir)
	return nil
}

func runHeftScanForImages(heftPath, chartURL, minConfidence string) ([]scanImage, error) {
	tmpDir, err := os.MkdirTemp("", "heft-e2e-scan-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	args := []string{"scan", chartURL, "--min-confidence=" + minConfidence}
	cmd := exec.Command(heftPath, args...)
	cmd.Dir = tmpDir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("heft scan failed: %v\noutput:\n%s", err, string(out))
	}

	var parsed scanOutput
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse scan output: %w", err)
	}

	return parsed.Images, nil
}

func resolveChartURL(chart artifactHubChart) (string, error) {
	if strings.TrimSpace(chart.ContentURL) != "" {
		return strings.TrimSpace(chart.ContentURL), nil
	}

	repositoryURL := strings.TrimSpace(chart.Repository.URL)
	if repositoryURL == "" {
		return "", fmt.Errorf("repository URL missing")
	}

	switch chart.Repository.Kind {
	case 0: // Helm repository
		return resolveFromHelmIndex(repositoryURL, firstNonEmpty(chart.NormalizedName, chart.Name), chart.Version)
	default:
		// Heuristic: derive OCI ref from repo URL if possible.
		if strings.HasPrefix(repositoryURL, "oci://") || strings.Contains(repositoryURL, "ghcr.io/") || strings.Contains(repositoryURL, "registry") {
			URL, err := ociURLFromRepo(repositoryURL, firstNonEmpty(chart.NormalizedName, chart.Name))
			if err != nil {
				return "", err
			}
			return URL, nil
		}
		return "", fmt.Errorf("unsupported repository kind %d and cannot infer oci ref", chart.Repository.Kind)
	}
}

func resolveFromHelmIndex(repoURL, chartName, version string) (string, error) {
	if chartName == "" {
		return "", fmt.Errorf("chart name is empty")
	}

	indexURL := strings.TrimRight(repoURL, "/") + "/index.yaml"

	response, err := http.Get(indexURL)
	if err != nil {
		return "", fmt.Errorf("fetch index.yaml: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("index.yaml status %d: %s", response.StatusCode, string(body))
	}

	var index struct {
		Entries map[string][]struct {
			Version string   `yaml:"version"`
			URLs    []string `yaml:"urls"`
		} `yaml:"entries"`
	}
	if err := yaml.NewDecoder(response.Body).Decode(&index); err != nil {
		return "", fmt.Errorf("parse index.yaml: %w", err)
	}

	entries, ok := index.Entries[chartName]
	if !ok || len(entries) == 0 {
		return "", fmt.Errorf("chart %s not found in index", chartName)
	}

	// If a specific version is requested, try to find it.
	if version != "" {
		for _, e := range entries {
			if e.Version == version && len(e.URLs) > 0 {
				return joinHelmURL(repoURL, e.URLs[0])
			}
		}
	}

	// Otherwise, pick the latest stable version if possible, falling back
	// to the latest pre-release when no stable versions exist.
	if len(entries) == 1 {
		if len(entries[0].URLs) == 0 {
			return "", fmt.Errorf("no URLs for chart %s", chartName)
		}
		return joinHelmURL(repoURL, entries[0].URLs[0])
	}

	type versionedEntry struct {
		major int
		minor int
		patch int
		pre   string
		isPre bool
		idx   int
	}

	parse := func(v string, idx int) versionedEntry {
		core := v
		pre := ""
		if dash := strings.Index(v, "-"); dash != -1 {
			core = v[:dash]
			pre = v[dash+1:]
		}
		parts := strings.Split(core, ".")
		ve := versionedEntry{idx: idx, pre: pre, isPre: pre != ""}
		if len(parts) > 0 {
			fmt.Sscanf(parts[0], "%d", &ve.major)
		}
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &ve.minor)
		}
		if len(parts) > 2 {
			fmt.Sscanf(parts[2], "%d", &ve.patch)
		}
		return ve
	}

	var stable, preReleases []versionedEntry
	for i, e := range entries {
		ve := parse(e.Version, i)
		if ve.isPre {
			preReleases = append(preReleases, ve)
		} else {
			stable = append(stable, ve)
		}
	}

	choose := func(list []versionedEntry) versionedEntry {
		best := list[0]
		for _, ve := range list[1:] {
			if ve.major != best.major {
				if ve.major > best.major {
					best = ve
				}
				continue
			}
			if ve.minor != best.minor {
				if ve.minor > best.minor {
					best = ve
				}
				continue
			}
			if ve.patch != best.patch {
				if ve.patch > best.patch {
					best = ve
				}
				continue
			}
			// For equal core versions, prefer non-pre over pre; for
			// both pre, just keep the existing best.
		}
		return best
	}

	var chosen versionedEntry
	if len(stable) > 0 {
		chosen = choose(stable)
	} else if len(preReleases) > 0 {
		chosen = choose(preReleases)
	} else {
		// Fallback: no parseable versions, use the first entry.
		chosen = versionedEntry{idx: 0}
	}

	best := entries[chosen.idx]
	if len(best.URLs) == 0 {
		return "", fmt.Errorf("no URLs for chart %s best version", chartName)
	}
	return joinHelmURL(repoURL, best.URLs[0])
}

func joinHelmURL(repositoryURL, chartURL string) (string, error) {
	repository, err := url.Parse(repositoryURL)
	if err != nil {
		return "", err
	}
	chart, err := url.Parse(chartURL)
	if err != nil {
		return "", err
	}
	return repository.ResolveReference(chart).String(), nil
}

func ociURLFromRepo(repoURL, chartName string) (string, error) {
	if chartName == "" {
		return "", fmt.Errorf("chart name is empty")
	}

	trimmed := strings.TrimPrefix(repoURL, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "oci://")
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "", fmt.Errorf("invalid repo URL %q", repoURL)
	}

	return "oci://" + trimmed + "/" + chartName, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeImageName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}

	// Extract first path segment.
	first := name
	if slash := strings.Index(first, "/"); slash != -1 {
		first = first[:slash]
	}

	// If the first segment has a dot, treat it as an explicit registry host.
	if strings.Contains(first, ".") {
		return name
	}

	// No explicit registry host; assume Docker Hub.

	// Has a slash: Docker Hub user/org image.
	if strings.Contains(name, "/") {
		base := name
		if !strings.Contains(base, ":") && !strings.Contains(base, "@") {
			base = base + ":latest"
		}
		return "docker.io/" + base
	}

	// No slash: Docker Hub library.
	base := name
	if !strings.Contains(base, ":") && !strings.Contains(base, "@") {
		base = base + ":latest"
	}
	return "docker.io/library/" + base
}
