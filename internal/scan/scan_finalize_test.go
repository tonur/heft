package scan

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestFinalizeScanResultNoImagesWithWarnings(t *testing.T) {
	t.Helper()

	warn := errors.New("detector failed")
	_, err := finalizeScanResult(nil, []error{warn}, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "detector failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeScanResultNoImagesNoWarnings(t *testing.T) {
	t.Helper()

	_, err := finalizeScanResult(nil, nil, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no images") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeScanResultLogsWarningsAndReturnsImages(t *testing.T) {
	prevWriter := logWriter
	defer func() { logWriter = prevWriter }()

	var buffer bytes.Buffer
	logWriter = &buffer

	images := []ImageFinding{{Name: "high", Confidence: ConfidenceHigh}}
	warnings := []error{errors.New("first"), errors.New("second")}

	result, err := finalizeScanResult(images, warnings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 1 || result.Images[0].Name != "high" {
		t.Fatalf("unexpected images: %+v", result.Images)
	}

	output := buffer.String()
	if !strings.Contains(output, "heft: warning:") || !strings.Contains(output, "first") || !strings.Contains(output, "second") {
		t.Fatalf("warnings not logged as expected: %q", output)
	}
}

func TestFinalizeScanResultAppliesMinConfidenceFilter(t *testing.T) {
	images := []ImageFinding{
		{Name: "high", Confidence: ConfidenceHigh},
		{Name: "medium", Confidence: ConfidenceMedium},
		{Name: "low", Confidence: ConfidenceLow},
	}

	result, err := finalizeScanResult(images, nil, ConfidenceHigh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 1 || result.Images[0].Name != "high" {
		t.Fatalf("expected only high confidence image, got %+v", result.Images)
	}

	result, err = finalizeScanResult(images, nil, ConfidenceMedium)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 2 {
		t.Fatalf("expected high and medium images, got %+v", result.Images)
	}

	result, err = finalizeScanResult(images, nil, ConfidenceLow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 3 {
		t.Fatalf("expected all images, got %+v", result.Images)
	}
}
