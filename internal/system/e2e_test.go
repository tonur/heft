//go:build e2e

package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildHeftBinary(t *testing.T) string {
	t.Helper()

	// Ensure helm is available since heft relies on it.
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

	return binPath
}

func repositoryRootDirectory(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return repoRoot
}

func runHeftScan(t *testing.T, binPath string, args ...string) []byte {
	t.Helper()

	temporaryDir := t.TempDir()
	command := exec.Command(binPath, args...)
	command.Env = os.Environ()
	command.Dir = temporaryDir

	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("heft scan failed: %v\noutput:\n%s", err, out)
	}

	return out
}

func TestHeftScanBasicChartEndToEnd(t *testing.T) {
	binaryPath := buildHeftBinary(t)
	repositoryRoot := repositoryRootDirectory(t)
	chartPath := filepath.Join(repositoryRoot, "internal", "scan", "testdata", "basic-chart")

	out := runHeftScan(t, binaryPath, "scan", chartPath, "--min-confidence=high")

	if !strings.Contains(string(out), "example.com/basic/app:1.2.3") {
		t.Fatalf("expected output to contain basic chart image, got:\n%s", out)
	}
}

func TestHeftScanBasicChartTGZEndToEnd(t *testing.T) {
	binaryPath := buildHeftBinary(t)
	repositoryRoot := repositoryRootDirectory(t)
	tgzPath := filepath.Join(repositoryRoot, "internal", "scan", "testdata", "basic-chart.tgz")

	out := runHeftScan(t, binaryPath, "scan", tgzPath, "--min-confidence=high")

	if !strings.Contains(string(out), "example.com/basic/app:1.2.3") {
		t.Fatalf("expected output to contain basic chart image from tgz, got:\n%s", out)
	}
}

func TestHeftScanBasicChartWithRemoteTGZ(t *testing.T) {
	binaryPath := buildHeftBinary(t)
	repositoryRoot := repositoryRootDirectory(t)
	tgzPath := filepath.Join(repositoryRoot, "internal", "scan", "testdata", "basic-chart.tgz")

	out := runHeftScan(t, binaryPath, "scan", tgzPath, "--min-confidence=high")

	if !strings.Contains(string(out), "example.com/basic/app:1.2.3") {
		t.Fatalf("expected output to contain basic chart image from remote-style tgz, got:\n%s", out)
	}
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
