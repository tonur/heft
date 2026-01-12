//go:build e2e

package system

import (
	"path/filepath"
	"strings"
	"testing"
)

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
