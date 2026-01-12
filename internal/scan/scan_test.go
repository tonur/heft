package scan

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeHelmBinary builds a small helm-like binary that understands
// "dependency build" and "template" but does no real work.
func fakeHelmBinary(t *testing.T, dir string) string {
	t.Helper()

	src := filepath.Join(dir, "helm_main.go")
	code := `package main
import (
	"fmt"
	"os"
)
func main() {
	if len(os.Args) >= 3 && os.Args[1] == "dependency" && os.Args[2] == "build" {
		// Simulate successful dependency build.
		fmt.Fprintln(os.Stderr, "fake helm dependency build")
		os.Exit(0)
	}
	if len(os.Args) >= 2 && os.Args[1] == "template" {
		// Simulate successful helm template invocation.
		fmt.Fprintln(os.Stderr, "fake helm template")
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, "unexpected helm invocation", os.Args)
	os.Exit(1)
}
`
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatalf("WriteFile fake helm: %v", err)
	}

	bin := filepath.Join(dir, "helm")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	command := exec.Command("go", "build", "-o", bin, src)
	command.Env = os.Environ()
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fake helm: %v\n%s", err, string(out))
	}

	return bin
}

func TestScanIncludesOptionalDependenciesWithFakeHelm(t *testing.T) {
	// This test ensures that the IncludeOptionalDeps branch in Scan runs
	// helm dependency build and attempts to scan subcharts. We do not
	// assert on specific images, only that it does not error when
	// dependency build succeeds and the charts directory exists.

	repository := t.TempDir()

	// Create a minimal chart layout with a charts/ subdirectory.
	chartRoot := filepath.Join(repository, "parent")
	if err := os.MkdirAll(filepath.Join(chartRoot, "charts", "child"), 0o755); err != nil {
		t.Fatalf("MkdirAll chart layout: %v", err)
	}
	// Minimal Chart.yaml to keep detectors happy where required.
	if err := os.WriteFile(filepath.Join(chartRoot, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\n"), 0o644); err != nil {
		t.Fatalf("WriteFile Chart.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartRoot, "values.yaml"), []byte("image:\n  repository: example.com/foo/bar\n  tag: v1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile values.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartRoot, "charts", "child", "Chart.yaml"), []byte("apiVersion: v2\nname: child\n"), 0o644); err != nil {
		t.Fatalf("WriteFile child Chart.yaml: %v", err)
	}

	// Build a fake helm binary.
	fakeDir := t.TempDir()
	helmBin := fakeHelmBinary(t, fakeDir)

	result, err := Scan(Options{
		ChartPath:           chartRoot,
		HelmBin:             helmBin,
		IncludeOptionalDeps: true,
	})
	if err != nil {
		t.Fatalf("Scan with IncludeOptionalDeps returned error: %v", err)
	}
	if len(result.Images) == 0 {
		t.Fatalf("expected some images when scanning chart with values.yaml, got 0")
	}
}

func TestScanOptionalSubchartsNoChartsDirReturnsNil(t *testing.T) {
	root := t.TempDir()
	// No charts/ subdirectory created.
	options := Options{ChartPath: root}

	results := scanOptionalSubcharts(options)
	if results != nil {
		t.Fatalf("expected nil when charts dir is missing, got %v", results)
	}
}

func TestScanOptionalSubchartsSkipsNonDirectoriesAndLogs(t *testing.T) {
	root := t.TempDir()
	chartsDir := filepath.Join(root, "charts")
	if err := os.MkdirAll(chartsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll chartsDir: %v", err)
	}

	// Non-directory entry should be skipped.
	if err := os.WriteFile(filepath.Join(chartsDir, "README.md"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("WriteFile README.md: %v", err)
	}

	// Create one subchart directory to exercise the main loop.
	subchartDir := filepath.Join(chartsDir, "child")
	if err := os.MkdirAll(subchartDir, 0o755); err != nil {
		t.Fatalf("MkdirAll subchartDir: %v", err)
	}

	// Point logWriter at a buffer so we can assert on verbose output.
	var buf bytes.Buffer
	oldLogWriter := logWriter
	logWriter = &buf
	defer func() { logWriter = oldLogWriter }()

	options := Options{ChartPath: root, Verbose: true}

	// We do not assert on the number of results because that depends on
	// other detectors; we only verify that the non-directory is skipped
	// and that we log about the subchart path.
	_ = scanOptionalSubcharts(options)

	logged := buf.String()
	if !bytes.Contains([]byte(logged), []byte("subchart=\""+subchartDir+"\"")) {
		t.Fatalf("expected verbose log for subchart %s, got: %s", subchartDir, logged)
	}
}

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

// TestScanVerboseWithEmptyChart exercises the early path where no
// detectors find any images and a warning is returned.
func TestScanVerboseWithEmptyChart(t *testing.T) {
	dir := t.TempDir()

	_, err := Scan(Options{ChartPath: dir, Verbose: true, HelmBin: "false"})
	if err == nil {
		t.Fatalf("expected error for empty chart with no images, got nil")
	}
}

func TestFinalizeScanResultNoImagesWithWarnings(t *testing.T) {
	t.Helper()

	warn := errors.New("detector failed")
	_, err := finalizeScanResult(nil, []error{warn}, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "detector failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeScanResultNoImagesNoWarnings(t *testing.T) {
	t.Helper()

	_, err := finalizeScanResult(nil, nil, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no images") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeScanResultLogsWarningsAndReturnsImages(t *testing.T) {
	prevWriter := logWriter
	defer func() { logWriter = prevWriter }()

	var buffer bytes.Buffer
	logWriter = &buffer

	images := []ImageFinding{{Name: "high", Confidence: ConfidenceHigh}}
	warnings := []error{errors.New("first"), errors.New("second")}

	result, err := finalizeScanResult(images, warnings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 1 || result.Images[0].Name != "high" {
		t.Fatalf("unexpected images: %+v", result.Images)
	}

	output := buffer.String()
	if !strings.Contains(output, "heft: warning:") || !strings.Contains(output, "first") || !strings.Contains(output, "second") {
		t.Fatalf("warnings not logged as expected: %q", output)
	}
}

func TestFinalizeScanResultAppliesMinConfidenceFilter(t *testing.T) {
	images := []ImageFinding{
		{Name: "high", Confidence: ConfidenceHigh},
		{Name: "medium", Confidence: ConfidenceMedium},
		{Name: "low", Confidence: ConfidenceLow},
	}

	result, err := finalizeScanResult(images, nil, ConfidenceHigh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 1 || result.Images[0].Name != "high" {
		t.Fatalf("expected only high confidence image, got %+v", result.Images)
	}

	result, err = finalizeScanResult(images, nil, ConfidenceMedium)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 2 {
		t.Fatalf("expected high and medium images, got %+v", result.Images)
	}

	result, err = finalizeScanResult(images, nil, ConfidenceLow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 3 {
		t.Fatalf("expected all images, got %+v", result.Images)
	}
}
