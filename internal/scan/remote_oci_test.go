package scan

import "testing"

func TestFetchAndExtractChartOCIErrorsWithoutHelm(t *testing.T) {
	// We only need to exercise the OCI branch enough to ensure it
	// attempts a helm pull; we do not depend on a real registry.
	if _, err := fetchAndExtractChart("oci://example.com/namespace/chart"); err == nil {
		t.Fatalf("expected error for OCI ref without helm, got nil")
	}
}
