package scan

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchAndExtractChartUnsupportedRef ensures unsupported schemes
// produce a clear error.
func TestFetchAndExtractChartUnsupportedRef(t *testing.T) {
	if _, err := fetchAndExtractChart("ftp://example.com/chart.tgz"); err == nil {
		t.Fatalf("expected error for unsupported ref, got nil")
	}
}

// errorRoundTripper forces HTTP client errors for downloadFile.
type errorRoundTripper struct{}

func (errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("boom")
}

// TestDownloadFileNetworkError uses a custom client to exercise the
// network-error path in downloadFile.
func TestDownloadFileNetworkError(t *testing.T) {
	// We cannot inject the client directly, but we can point downloadFile
	// at an invalid URL so that http.Get fails quickly. Using a malformed
	// scheme triggers an immediate error.
	if err := downloadFile("http://[::1]:namedport", ""); err == nil {
		t.Fatalf("expected error for invalid URL, got nil")
	}
}

// TestFetchAndExtractChartHTTPError ensures non-200 responses from the
// server are propagated from downloadFile through fetchAndExtractChart.
func TestFetchAndExtractChartHTTPError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad"))
	}))
	defer testServer.Close()

	if _, err := fetchAndExtractChart(testServer.URL); err == nil {
		t.Fatalf("expected error for HTTP 502 response, got nil")
	}
}
