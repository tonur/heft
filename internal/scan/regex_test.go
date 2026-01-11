package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRegexEmptyChartPath(t *testing.T) {
	if _, err := detectRegex(Options{ChartPath: ""}); err == nil {
		t.Fatalf("expected error for empty chart path, got nil")
	}
}

func TestDetectRegexSkipsTestsAndSnapshotsAndFiltersJunk(t *testing.T) {
	root := t.TempDir()

	// File that may be scanned and potentially produce results.
	valuesPath := filepath.Join(root, "values.yaml")
	if err := os.WriteFile(valuesPath, []byte("image: nginx:1.2.3"), 0o644); err != nil {
		t.Fatalf("WriteFile values.yaml: %v", err)
	}

	// File under tests/ should be skipped entirely.
	testsDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll tests: %v", err)
	}
	testFile := filepath.Join(testsDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte("image: alpine:3.18"), 0o644); err != nil {
		t.Fatalf("WriteFile test.yaml: %v", err)
	}

	// File under a snapshot-like directory should also be skipped.
	snapDir := filepath.Join(root, "__snapshot__")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatalf("MkdirAll snapshot: %v", err)
	}
	snapFile := filepath.Join(snapDir, "snap.yaml")
	if err := os.WriteFile(snapFile, []byte("image: busybox:latest"), 0o644); err != nil {
		t.Fatalf("WriteFile snap.yaml: %v", err)
	}

	// File with junk patterns that should be filtered out.
	junkPath := filepath.Join(root, "junk.yaml")
	junkContent := "image: foo:.\nother: bar=.\nvalid: redis:6.0"
	if err := os.WriteFile(junkPath, []byte(junkContent), 0o644); err != nil {
		t.Fatalf("WriteFile junk.yaml: %v", err)
	}

	results, err := detectRegex(Options{ChartPath: root})
	if err != nil {
		t.Fatalf("detectRegex error: %v", err)
	}

	for _, r := range results {
		if r.File == testFile || r.File == snapFile {
			t.Fatalf("unexpected match in skipped file %s", r.File)
		}
		if r.Name == "foo:." || r.Name == "bar=." {
			t.Fatalf("junk pattern %q should have been filtered out", r.Name)
		}
	}
}
