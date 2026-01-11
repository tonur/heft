package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
