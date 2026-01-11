package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

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

func selectExpectedImages(images []scanImage) []expectedImage {
	var highConfidence []expectedImage
	for _, img := range images {
		if img.Confidence != "high" {
			continue
		}
		highConfidence = append(highConfidence, expectedImage{
			Image:      normalizeImageName(img.Name),
			Confidence: img.Confidence,
			Source:     img.Source,
		})
	}

	if len(highConfidence) > 0 {
		return highConfidence
	}

	var all []expectedImage
	for _, img := range images {
		all = append(all, expectedImage{
			Image:      normalizeImageName(img.Name),
			Confidence: img.Confidence,
			Source:     img.Source,
		})
	}

	return all
}

// scaffoldChart writes chart metadata and a command fixture for the
// given Artifact Hub chart using the provided heft binary.
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

	if err := writeChartMetadata(metadataPath, &md); err != nil {
		return err
	}

	images, err := runHeftScanForImages(heftPath, chartURL, minConfidence)
	if err != nil {
		return fmt.Errorf("heft scan: %w", err)
	}

	expected := selectExpectedImages(images)

	fixture := commandFixture{
		Name:           "min-confidence-" + minConfidence,
		Arguments:      []string{"scan", "${CHART_URL}", "--min-confidence=" + minConfidence},
		ExpectedImages: expected,
	}

	if err := writeCommandFixture(chartDir, &fixture); err != nil {
		return err
	}

	fmt.Printf("Scaffolded chart %s (%s) -> %s\n", md.Name, md.Version, chartDir)
	return nil
}

// writeFileFunc is a variable to allow tests to stub file writes.
var writeFileFunc = os.WriteFile

func writeChartMetadata(metadataPath string, md *chartMetadata) error {
	mdBytes, err := yaml.Marshal(md)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := writeFileFunc(metadataPath, mdBytes, 0o644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	return nil
}

func writeCommandFixture(chartDir string, fixture *commandFixture) error {
	fixtureBytes, err := yaml.Marshal(fixture)
	if err != nil {
		return fmt.Errorf("marshal command fixture: %w", err)
	}

	fixturePath := filepath.Join(chartDir, "commands", fixture.Name+".yaml")
	if err := os.WriteFile(fixturePath, fixtureBytes, 0o644); err != nil {
		return fmt.Errorf("write command fixture: %w", err)
	}
	return nil
}

func updateChartMetadataAndCheckDrift(chartDir, metadataPath string, chart *artifactHubChart, chartURL, heftPath string) error {
	md := chartMetadata{
		Name:    firstNonEmpty(chart.NormalizedName, chart.Name),
		URL:     chartURL,
		Version: chart.Version,
		Source:  "artifacthub",
	}

	if err := writeChartMetadata(metadataPath, &md); err != nil {
		return err
	}

	images, err := runHeftScanForImages(heftPath, chartURL, "high")
	if err != nil {
		return fmt.Errorf("heft scan: %w", err)
	}

	expected := selectExpectedImages(images)

	newFixture := commandFixture{
		Name:           "min-confidence-high",
		Arguments:      []string{"scan", "${CHART_URL}", "--min-confidence=high"},
		ExpectedImages: expected,
	}

	existingPath := filepath.Join(chartDir, "commands", newFixture.Name+".yaml")
	data, err := os.ReadFile(existingPath)
	if err != nil {
		// If there is no existing fixture, nothing to compare.
		return nil
	}

	var existingFixture commandFixture
	if err := yaml.Unmarshal(data, &existingFixture); err != nil {
		return fmt.Errorf("parse existing command fixture: %w", err)
	}

	if !reflect.DeepEqual(existingFixture, newFixture) {
		fmt.Fprintf(os.Stderr, "Warning: drift detected for chart %s (%s) in %s\n", md.Name, md.Version, existingPath)
	}

	return nil
}

// runHeftScanForImages executes the heft binary against the given
// chart URL and parses its YAML output into scanImage values.
func runHeftScanForImages(heftPath, chartURL, minConfidence string) ([]scanImage, error) {
	return runHeftScanForImagesFunc(heftPath, chartURL, minConfidence)
}

// runHeftScanForImagesFunc is a function variable to allow tests to
// stub out the external heft invocation.
var runHeftScanForImagesFunc = func(heftPath, chartURL, minConfidence string) ([]scanImage, error) {
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

// resolveChartURL derives a fetchable URL or OCI reference for the
// given Artifact Hub chart.
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
