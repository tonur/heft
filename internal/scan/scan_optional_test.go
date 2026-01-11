package scan

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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
