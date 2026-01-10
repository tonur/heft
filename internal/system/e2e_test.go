//go:build e2e

package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHeftScanBasicChartEndToEnd(t *testing.T) {
	// Ensure helm is available since heft relies on it.
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available in PATH; skipping e2e test")
	}

	// Build heft binary in a temp dir.
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "heft")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/heft")
	build.Env = os.Environ()
	// From internal/system, repo root is two levels up.
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build heft: %v\n%s", err, out)
	}

	// Resolve absolute path to the basic test chart directory.
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	chartPath := filepath.Join(repoRoot, "internal", "scan", "testdata", "basic-chart")

	cmd := exec.Command(binPath, "scan", chartPath, "--min-confidence=high")
	cmd.Env = os.Environ()
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("heft scan failed: %v\noutput:\n%s", err, out)
	}

	// Basic assertion: the expected image string should appear in output.
	if !strings.Contains(string(out), "example.com/basic/app:1.2.3") {
		t.Fatalf("expected output to contain basic chart image, got:\n%s", out)
	}
}

func TestHeftScanBasicChartTGZEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available in PATH; skipping e2e test")
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "heft")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/heft")
	build.Env = os.Environ()
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build heft: %v\n%s", err, out)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	tgzPath := filepath.Join(repoRoot, "internal", "scan", "testdata", "basic-chart.tgz")

	cmd := exec.Command(binPath, "scan", tgzPath, "--min-confidence=high")
	cmd.Env = os.Environ()
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("heft scan (tgz) failed: %v\noutput:\n%s", err, out)
	}

	if !strings.Contains(string(out), "example.com/basic/app:1.2.3") {
		t.Fatalf("expected output to contain basic chart image from tgz, got:\n%s", out)
	}
}

func TestHeftScanBasicChartWithRemoteTGZ(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available in PATH; skipping e2e test")
	}

	// This test uses a local .tgz as a "remote" style ref to ensure that
	// passing an archive directly as the chart-ref works end-to-end.
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "heft")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/heft")
	build.Env = os.Environ()
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build heft: %v\n%s", err, out)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	tgzPath := filepath.Join(repoRoot, "internal", "scan", "testdata", "basic-chart.tgz")

	cmd := exec.Command(binPath, "scan", tgzPath, "--min-confidence=high")
	cmd.Env = os.Environ()
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("heft scan with remote-style tgz failed: %v\noutput:\n%s", err, out)
	}

	if !strings.Contains(string(out), "example.com/basic/app:1.2.3") {
		t.Fatalf("expected output to contain basic chart image from remote-style tgz, got:\n%s", out)
	}
}

type urlChartTestCase struct {
	Name           string
	Command        string
	ExpectedImages []expectedImage
}

type expectedImage struct {
	Image      string
	Confidence string
	Source     string
}

type scanImage struct {
	Name       string `yaml:"name"`
	Confidence string `yaml:"confidence"`
	Source     string `yaml:"source"`
}

type scanOutput struct {
	Images []scanImage `yaml:"images"`
}

func runHeftURLChartTests(t *testing.T, binPath, repoRoot string, cases []urlChartTestCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if len(tc.ExpectedImages) == 0 {
				t.Fatal("ExpectedImages must be set")
			}

			tmpDir := t.TempDir()

			// Each test case specifies a full command string
			// (e.g. "scan <url> --include-optional-deps").
			cmdStr := tc.Command
			if cmdStr == "" {
				t.Fatalf("command must be set for %s", tc.Name)
			}
			parts := strings.Fields(cmdStr)
			if len(parts) == 0 {
				t.Fatalf("invalid command for %s", tc.Name)
			}
			cmd := exec.Command(binPath, parts...)
			cmd.Env = os.Environ()
			cmd.Dir = tmpDir

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("heft scan for %s failed: %v\noutput:\n%s", tc.Name, err, out)
			}

			var parsed scanOutput
			if err := yaml.Unmarshal(out, &parsed); err != nil {
				t.Fatalf("failed to parse YAML output for %s: %v\noutput:\n%s", tc.Name, err, out)
			}

			for _, expected := range tc.ExpectedImages {
				if expected.Image == "" {
					continue
				}
				found := false
				for _, img := range parsed.Images {
					if img.Name == expected.Image && img.Confidence == expected.Confidence && img.Source == expected.Source {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected image not found for %s: image=%q confidence=%q source=%q\nparsed=%+v", tc.Name, expected.Image, expected.Confidence, expected.Source, parsed)
				}
			}
		})
	}
}

func TestHeftScanURLChartsEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available in PATH; skipping e2e test")
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "heft")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/heft")
	build.Env = os.Environ()
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build heft: %v\n%s", err, out)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}

	cases := []urlChartTestCase{
		{
			Name:    "external-secrets",
			Command: "scan https://github.com/external-secrets/external-secrets/releases/download/helm-chart-1.2.1/external-secrets-1.2.1.tgz --include-optional-deps --min-confidence=high",
			ExpectedImages: []expectedImage{
				{
					Image:      "ghcr.io/external-secrets/external-secrets:v1.2.1",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "ghcr.io/external-secrets/bitwarden-sdk-server:v0.5.2",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
			},
		},
		{
			Name:    "cilium",
			Command: "scan https://helm.cilium.io/cilium-1.18.5.tgz --min-confidence=high",
			ExpectedImages: []expectedImage{
				{
					Image:      "quay.io/cilium/cilium-envoy:v1.34.12-1765374555-6a93b0bbba8d6dc75b651cbafeedb062b2997716@sha256:3108521821c6922695ff1f6ef24b09026c94b195283f8bfbfc0fa49356a156e1",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "quay.io/cilium/cilium:v1.18.5@sha256:2c92fb05962a346eaf0ce11b912ba434dc10bd54b9989e970416681f4a069628",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "quay.io/cilium/operator-generic:v1.18.5@sha256:36c3f6f14c8ced7f45b40b0a927639894b44269dd653f9528e7a0dc363a4eb99",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
			},
		},
		{
			Name:    "prometheus",
			Command: "scan oci://ghcr.io/prometheus-community/charts/prometheus --min-confidence=high",
			ExpectedImages: []expectedImage{
				{
					Image:      "quay.io/prometheus-operator/prometheus-config-reloader:v0.88.0",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "quay.io/prometheus/alertmanager:v0.30.0",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "quay.io/prometheus/node-exporter:v1.10.2",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "quay.io/prometheus/prometheus:v3.9.1",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "quay.io/prometheus/pushgateway:v1.11.2",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
				{
					Image:      "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.17.0",
					Confidence: "high",
					Source:     "rendered-manifest",
				},
			},
		},
	}

	runHeftURLChartTests(t, binPath, repoRoot, cases)
}
