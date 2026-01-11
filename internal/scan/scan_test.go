package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSplitRepoAndTag(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		repository   string
		hasTag bool
	}{
		{"noTag", "ghcr.io/external-secrets/external-secrets", "ghcr.io/external-secrets/external-secrets", false},
		{"withTag", "ghcr.io/external-secrets/external-secrets:v1.2.1", "ghcr.io/external-secrets/external-secrets", true},
		{"withPortAndTag", "registry:5000/ns/app:v1", "registry:5000/ns/app", true},
		{"withDigest", "ghcr.io/ns/app@sha256:abcd", "ghcr.io/ns/app", true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repository, hasTag := splitRepoAndTag(testCase.input)
			if repository != testCase.repository || hasTag != testCase.hasTag {
				t.Fatalf("splitRepoAndTag(%q) = (%q,%v), want (%q,%v)", testCase.input, repository, hasTag, testCase.repository, testCase.hasTag)
			}
		})
	}
}

func TestDedupeImagesPrefersHigherConfidenceAndTagged(t *testing.T) {
	images := []ImageFinding{
		{Name: "ghcr.io/external-secrets/external-secrets", Confidence: ConfidenceMedium},
		{Name: "ghcr.io/external-secrets/external-secrets:v1.2.1", Confidence: ConfidenceHigh},
		{Name: "ghcr.io/external-secrets/external-secrets:v1.0.0", Confidence: ConfidenceLow},
	}

	got := dedupeImages(images)
	if len(got) != 1 {
		t.Fatalf("expected 1 image after dedupe, got %d", len(got))
	}
	if got[0].Name != "ghcr.io/external-secrets/external-secrets:v1.2.1" || got[0].Confidence != ConfidenceHigh {
		t.Fatalf("unexpected result: %+v", got[0])
	}
}

func TestDedupeImagesPrefersTaggedAtSameConfidence(t *testing.T) {
	images := []ImageFinding{
		{Name: "example.com/foo/bar", Confidence: ConfidenceMedium},
		{Name: "example.com/foo/bar:latest", Confidence: ConfidenceMedium},
	}

	got := dedupeImages(images)
	if len(got) != 1 {
		t.Fatalf("expected 1 image after dedupe, got %d", len(got))
	}
	if got[0].Name != "example.com/foo/bar:latest" {
		t.Fatalf("expected tagged variant to win, got %q", got[0].Name)
	}
}

func TestConfidenceFilter(t *testing.T) {
	images := []ImageFinding{
		{Name: "high", Confidence: ConfidenceHigh},
		{Name: "medium", Confidence: ConfidenceMedium},
		{Name: "low", Confidence: ConfidenceLow},
	}

	// We call the filtering logic indirectly via Scan by constructing options
	// and stubbing out detectors would be heavy here. Instead, we rely on
	// dedupeImages behavior and manually apply the same logic used in Scan.
	result := dedupeImages(images)

	filter := func(min Confidence) []ImageFinding {
		out := result[:0]
		for _, image := range result {
			if image.Confidence == ConfidenceHigh || (min == ConfidenceMedium && image.Confidence == ConfidenceMedium) || min == ConfidenceLow {
				out = append(out, image)
			}
		}
		return out
	}

	high := filter(ConfidenceHigh)
	if len(high) != 1 || high[0].Name != "high" {
		t.Fatalf("expected only high confidence, got %+v", high)
	}

	medium := filter(ConfidenceMedium)
	if len(medium) != 2 { // high + medium
		t.Fatalf("expected 2 images for medium filter, got %d", len(medium))
	}

	low := filter(ConfidenceLow)
	if len(low) != 3 {
		t.Fatalf("expected all images for low filter, got %d", len(low))
	}
}

func TestCollectStaticImagesFromValuesYAML(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "values.yaml")
	content := []byte("image:\n  repository: example.com/foo/bar\n  tag: v1\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write temp values.yaml: %v", err)
	}

	images, err := detectStatic(Options{ChartPath: directory})
	if err != nil {
		t.Fatalf("detectStatic returned error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image from static detection, got %d", len(images))
	}
	if images[0].Name != "example.com/foo/bar:v1" || images[0].Confidence != ConfidenceMedium {
		t.Fatalf("unexpected static image result: %+v", images[0])
	}
}

func TestDetectRegexSkipsTestsAndJunkTags(t *testing.T) {
	directory := t.TempDir()

	// This file is under tests/ and should be ignored entirely.
	testDirectory := filepath.Join(directory, "tests")
	if err := os.Mkdir(testDirectory, 0o755); err != nil {
		t.Fatalf("failed to create tests dir: %v", err)
	}
	testFile := filepath.Join(testDirectory, "junk.yaml")
	if err := os.WriteFile(testFile, []byte("image: io/custom/app:.\n"), 0o644); err != nil {
		t.Fatalf("failed to write test junk file: %v", err)
	}

	// This file should be scanned, but the pattern ends with ":.", which
	// our detector treats as junk and filters out.
	goodFile := filepath.Join(directory, "values.yaml")
	if err := os.WriteFile(goodFile, []byte("image: example.com/foo/bar:.\n"), 0o644); err != nil {
		t.Fatalf("failed to write values file: %v", err)
	}

	images, err := detectRegex(Options{ChartPath: directory})
	if err != nil {
		t.Fatalf("detectRegex returned error: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("expected no regex images due to junk filtering, got %d", len(images))
	}
}

func TestScanWithBasicChartIntegration(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available in PATH; skipping integration test")
	}

	chartPath := filepath.Join("testdata", "basic-chart")
	images, err := Scan(Options{ChartPath: chartPath, HelmBin: "helm"})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(images.Images) != 1 {
		t.Fatalf("expected 1 image from basic chart, got %d", len(images.Images))
	}
	image := images.Images[0]
	if image.Name != "example.com/basic/app:1.2.3" {
		t.Fatalf("unexpected image name: %q", image.Name)
	}
	if image.Confidence != ConfidenceHigh || image.Source != SourceRendered {
		t.Fatalf("expected high-confidence rendered-manifest, got %+v", image)
	}
}

func TestScanMinConfidenceAndEmptyChart(t *testing.T) {
	directory := t.TempDir()

	// Empty directory: no static/regex images.
	_, err := Scan(Options{ChartPath: directory, MinConfidence: ConfidenceLow, HelmBin: "false"})
	if err == nil {
		t.Fatalf("expected error for low confidence with no images, got nil")
	}

	// High min-confidence on same empty directory should also error.
	_, err = Scan(Options{ChartPath: directory, MinConfidence: ConfidenceHigh, HelmBin: "false"})
	if err == nil {
		t.Fatalf("expected error for high confidence with no images, got nil")
	}
}
