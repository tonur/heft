package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWriteChartMetadataWritesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chart_metadata.yaml")

	md := &chartMetadata{Name: "chart", URL: "https://example.com", Version: "1.0.0", Source: "artifacthub"}
	if err := writeChartMetadata(path, md); err != nil {
		t.Fatalf("writeChartMetadata returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading metadata file failed: %v", err)
	}

	var decoded chartMetadata
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal metadata YAML: %v", err)
	}
	if decoded.Name != md.Name || decoded.URL != md.URL || decoded.Version != md.Version || decoded.Source != md.Source {
		t.Fatalf("decoded metadata %+v does not match original %+v", decoded, *md)
	}
}

func TestWriteCommandFixtureWritesYAML(t *testing.T) {
	dir := t.TempDir()
	fixture := &commandFixture{
		Name:      "min-confidence-high",
		Arguments: []string{"scan", "${CHART_URL}", "--min-confidence=high"},
		ExpectedImages: []expectedImage{
			{Image: "nginx:1.0", Confidence: "high", Source: "static"},
		},
	}

	// In production, scaffoldChart creates the commands directory before
	// calling writeCommandFixture. Mirror that here.
	if err := os.MkdirAll(filepath.Join(dir, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}

	if err := writeCommandFixture(dir, fixture); err != nil {
		t.Fatalf("writeCommandFixture returned error: %v", err)
	}

	path := filepath.Join(dir, "commands", fixture.Name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading command fixture failed: %v", err)
	}

	var decoded commandFixture
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal command fixture YAML: %v", err)
	}
	if decoded.Name != fixture.Name || len(decoded.Arguments) != len(fixture.Arguments) || len(decoded.ExpectedImages) != len(fixture.ExpectedImages) {
		t.Fatalf("decoded fixture %+v does not match original %+v", decoded, *fixture)
	}
}

func TestUpdateChartMetadataAndCheckDriftIdenticalFixtureNoWarning(t *testing.T) {
	dir := t.TempDir()
	chartDir := dir
	metadataPath := filepath.Join(dir, "chart_metadata.yaml")

	chart := &artifactHubChart{
		Name:    "mychart",
		Version: "1.0.0",
	}
	chartURL := "https://example.com/charts/mychart-1.0.0.tgz"
	heftPath := "heft" // not used because we will provide an existing fixture

	// Prepare an existing fixture matching what updateChartMetadataAndCheckDrift would produce.
	existing := commandFixture{
		Name:      "min-confidence-high",
		Arguments: []string{"scan", "${CHART_URL}", "--min-confidence=high"},
		ExpectedImages: []expectedImage{
			{Image: "nginx:1.0", Confidence: "high", Source: "static"},
		},
	}

	if err := os.MkdirAll(filepath.Join(chartDir, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	data, err := yaml.Marshal(&existing)
	if err != nil {
		t.Fatalf("marshal existing fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "commands", existing.Name+".yaml"), data, 0o644); err != nil {
		t.Fatalf("write existing fixture: %v", err)
	}

	oldRunHeft := runHeftScanForImagesFunc
	defer func() { runHeftScanForImagesFunc = oldRunHeft }()

	runHeftScanForImagesFunc = func(heftPath, chartURL, minConfidence string) ([]scanImage, error) {
		return []scanImage{{Name: "nginx:1.0", Confidence: "high", Source: "static"}}, nil
	}

	if err := updateChartMetadataAndCheckDrift(chartDir, metadataPath, chart, chartURL, heftPath); err != nil {
		t.Fatalf("updateChartMetadataAndCheckDrift returned error: %v", err)
	}
}

func TestUpdateChartMetadataAndCheckDriftMalformedExistingFixture(t *testing.T) {
	dir := t.TempDir()
	chartDir := dir
	metadataPath := filepath.Join(dir, "chart_metadata.yaml")

	chart := &artifactHubChart{
		Name:    "mychart",
		Version: "1.0.0",
	}
	chartURL := "https://example.com/charts/mychart-1.0.0.tgz"
	heftPath := "heft" // not used because we will fail before comparing

	if err := os.MkdirAll(filepath.Join(chartDir, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "commands", "min-confidence-high.yaml"), []byte("not: [yaml"), 0o644); err != nil {
		t.Fatalf("write malformed fixture: %v", err)
	}

	oldRunHeft := runHeftScanForImagesFunc
	defer func() { runHeftScanForImagesFunc = oldRunHeft }()

	runHeftScanForImagesFunc = func(heftPath, chartURL, minConfidence string) ([]scanImage, error) {
		return nil, nil
	}

	if err := updateChartMetadataAndCheckDrift(chartDir, metadataPath, chart, chartURL, heftPath); err == nil {
		t.Fatalf("expected error from malformed existing fixture")
	}
}
