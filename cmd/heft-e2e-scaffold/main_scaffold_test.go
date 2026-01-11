package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestScaffoldChartCreatesFixture uses a fake heft binary that prints a
// deterministic YAML payload so we can verify the generated metadata and
// command fixture from scaffoldChart.
func TestScaffoldChartCreatesFixture(t *testing.T) {
	tdir, err := os.MkdirTemp("", "heft-scaffold-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tdir)

	// Create a fake heft binary that prints a single high-confidence image.
	fakeHeft := filepath.Join(tdir, "heft")
	fakeSrc := filepath.Join(tdir, "main.go")
	if err := os.WriteFile(fakeSrc, []byte(`package main
import "fmt"
func main() {
	fmt.Println("images:\n- name: alpine\n  confidence: high\n  source: rendered-manifest")
}
`), 0o644); err != nil {
		t.Fatalf("write fake heft source: %v", err)
	}

	buildCmd := exec.Command("go", "build", "-o", fakeHeft, fakeSrc)
	buildCmd.Env = os.Environ()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake heft: %v\n%s", err, string(out))
	}
	if runtime.GOOS == "windows" {
		fakeHeft += ".exe"
	}

	chartDir := filepath.Join(tdir, "chart")
	metadataPath := filepath.Join(chartDir, "chart_metadata.yaml")

	chart := &artifactHubChart{
		Name:    "testchart",
		Version: "1.0.0",
	}

	if err := scaffoldChart(chartDir, metadataPath, chart, "dummy-chart-url", fakeHeft, "high"); err != nil {
		t.Fatalf("scaffoldChart error: %v", err)
	}

	// Ensure metadata file exists.
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("expected metadata file at %s: %v", metadataPath, err)
	}

	// Ensure a command fixture exists with the expected normalized image.
	commandsDir := filepath.Join(chartDir, "commands")
	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		t.Fatalf("ReadDir commands: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one command fixture in %s", commandsDir)
	}

	foundExpected := false
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(commandsDir, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), err)
		}
		if strings.Contains(string(content), "docker.io/library/alpine:latest") {
			foundExpected = true
			break
		}
	}

	if !foundExpected {
		t.Fatalf("expected commands fixture to contain docker.io/library/alpine:latest")
	}
}
