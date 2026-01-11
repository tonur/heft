package scan

import "testing"

// TestScanVerboseWithEmptyChart exercises the early path where no
// detectors find any images and a warning is returned.
func TestScanVerboseWithEmptyChart(t *testing.T) {
	dir := t.TempDir()

	_, err := Scan(Options{ChartPath: dir, Verbose: true, HelmBin: "false"})
	if err == nil {
		t.Fatalf("expected error for empty chart with no images, got nil")
	}
}
