package main

import (
	"errors"
	"os"
	"testing"
)

// TestWriteChartMetadataWriteError uses the writeFileFunc hook to
// exercise the error path when writing the metadata file fails.
func TestWriteChartMetadataWriteError(t *testing.T) {
	oldWrite := writeFileFunction
	defer func() { writeFileFunction = oldWrite }()

	writeFileFunction = func(filename string, data []byte, perm os.FileMode) error {
		return errors.New("disk full")
	}

	if err := writeChartMetadata("/tmp/chart_metadata.yaml", &chartMetadata{Name: "chart"}); err == nil {
		t.Fatalf("expected error from writeChartMetadata when writeFileFunc fails")
	}
}
