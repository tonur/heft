package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWriteChartMetadataAndReadBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chart_metadata.yaml")
	md := &chartMetadata{
		Name:    "demo",
		URL:     "https://example.com/demo-1.0.0.tgz",
		Version: "1.0.0",
		Source:  "artifacthub",
	}

	if err := writeChartMetadata(path, md); err != nil {
		t.Fatalf("writeChartMetadata error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var decoded chartMetadata
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(*md, decoded) {
		t.Fatalf("metadata mismatch: got %+v, want %+v", decoded, *md)
	}
}

func TestWriteCommandFixtureAndReadBack(t *testing.T) {
	dir := t.TempDir()

	// writeCommandFixture expects the commands directory to exist.
	if err := os.MkdirAll(filepath.Join(dir, "commands"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	fixture := &commandFixture{
		Name:      "min-confidence-high",
		Arguments: []string{"scan", "${CHART_URL}", "--min-confidence=high"},
		ExpectedImages: []expectedImage{{
			Image:      "alpine",
			Confidence: "high",
			Source:     "rendered-manifest",
		}},
	}

	if err := writeCommandFixture(dir, fixture); err != nil {
		t.Fatalf("writeCommandFixture error: %v", err)
	}

	path := filepath.Join(dir, "commands", "min-confidence-high.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var decoded commandFixture
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Compare field by field instead of pointer equality to avoid
	// any subtle differences in zero values.
	if fixture.Name != decoded.Name {
		t.Fatalf("name mismatch: got %q, want %q", decoded.Name, fixture.Name)
	}
	if !reflect.DeepEqual(fixture.Arguments, decoded.Arguments) {
		t.Fatalf("arguments mismatch: got %#v, want %#v", decoded.Arguments, fixture.Arguments)
	}
	if !reflect.DeepEqual(fixture.ExpectedImages, decoded.ExpectedImages) {
		t.Fatalf("expectedImages mismatch: got %#v, want %#v", decoded.ExpectedImages, fixture.ExpectedImages)
	}
}

func TestUpdateChartMetadataAndCheckDrift_NoExistingFixture(t *testing.T) {
	repo := t.TempDir()
	metadataPath := filepath.Join(repo, "chart_metadata.yaml")

	chart := &artifactHubChart{
		Name:           "demo",
		NormalizedName: "demo",
		Version:        "1.2.3",
	}

	// Create commands directory but intentionally omit the high fixture.
	if err := os.MkdirAll(filepath.Join(repo, "commands"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// First call: should attempt to update metadata even if scan fails.
	_ = updateChartMetadataAndCheckDrift(repo, metadataPath, chart, "https://example.com/demo-1.2.3.tgz", "/nonexistent/heft")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("ReadFile metadata: %v", err)
	}
	if !bytes.Contains(data, []byte("version: 1.2.3")) {
		t.Fatalf("metadata does not contain updated version: %s", string(data))
	}
}
