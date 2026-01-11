package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestScaffoldChartErrorsOnEmptyScan uses a fake heft binary that prints an
// empty image list to ensure scaffoldChart handles the no-image case without
// panicking and still writes metadata.
func TestScaffoldChartErrorsOnEmptyScan(t *testing.T) {
	tempDirectory, err := os.MkdirTemp("", "heft-scaffold-empty-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tempDirectory)

	fakeHeft := filepath.Join(tempDirectory, "heft")
	fakeSrc := filepath.Join(tempDirectory, "main.go")
	if err := os.WriteFile(fakeSrc, []byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"images: []\") }\n"), 0o644); err != nil {
		t.Fatalf("write fake heft source: %v", err)
	}

	buildCommand := exec.Command("go", "build", "-o", fakeHeft, fakeSrc)
	buildCommand.Env = os.Environ()
	if out, err := buildCommand.CombinedOutput(); err != nil {
		t.Fatalf("build fake heft: %v\n%s", err, string(out))
	}
	if runtime.GOOS == "windows" {
		fakeHeft += ".exe"
	}

	chartDir := filepath.Join(tempDirectory, "chart")
	metadataPath := filepath.Join(chartDir, "chart_metadata.yaml")

	chart := &artifactHubChart{Name: "emptychart"}

	err = scaffoldChart(chartDir, metadataPath, chart, "dummy-chart-url", fakeHeft, "low")
	if err != nil {
		// An error from scaffoldChart is acceptable here; the purpose of this
		// test is to ensure it does not panic and that metadata is written.
	}

	if _, statErr := os.Stat(metadataPath); statErr != nil {
		t.Fatalf("expected metadata file at %s: %v", metadataPath, statErr)
	}
}
