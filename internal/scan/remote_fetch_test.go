package scan

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFetchAndExtractChartHTTPSuccess verifies the HTTP branch of
// fetchAndExtractChart, including downloadFile and extractTarGz
// integration.
func TestFetchAndExtractChartHTTPSuccess(t *testing.T) {
	// Build a small .tgz archive in a temp file.
	tmpDir := t.TempDir()
	tgzPath := filepath.Join(tmpDir, "chart.tgz")
	f, err := os.Create(tgzPath)
	if err != nil {
		t.Fatalf("failed to create tgz: %v", err)
	}
	gz := gzip.NewWriter(f)
	tarWriter := tar.NewWriter(gz)

	// Write a single file mychart/Chart.yaml
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "mychart/Chart.yaml",
		Mode:     0o644,
		Size:     int64(len("name: mychart\n")),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tarWriter.Write([]byte("name: mychart\n")); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	// Serve the tgz over HTTP.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, tgzPath)
	}))
	defer server.Close()

	root, err := fetchAndExtractChart(server.URL)
	if err != nil {
		t.Fatalf("fetchAndExtractChart returned error: %v", err)
	}

	// Root should point to a directory containing Chart.yaml.
	info, err := os.Stat(filepath.Join(root, "Chart.yaml"))
	if err != nil {
		t.Fatalf("expected Chart.yaml in extracted root: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected Chart.yaml to be a file, got directory")
	}
}

// TestFetchAndExtractChartOCIFailureWithStub uses the helmPullCommand
// hook to exercise the OCI error path deterministically.
func TestFetchAndExtractChartOCIFailureWithStub(t *testing.T) {
	old := helmPullCommand
	defer func() { helmPullCommand = old }()

	helmPullCommand = func(ref, tmpDir string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 1")
	}

	if _, err := fetchAndExtractChart("oci://example.com/mychart"); err == nil {
		t.Fatalf("expected error for failing OCI helm pull")
	}
}

// TestFetchAndExtractChartOCISuccessWithStub uses the helmPullCommand
// hook to exercise the successful OCI path by creating a fake chart
// directory under tmpDir.
func TestFetchAndExtractChartOCISuccessWithStub(t *testing.T) {
	old := helmPullCommand
	defer func() { helmPullCommand = old }()

	helmPullCommand = func(ref, tmpDir string) *exec.Cmd {
		// Create a fake chart directory that fetchAndExtractChart will
		// discover after the stubbed helm command "succeeds".
		if err := os.MkdirAll(filepath.Join(tmpDir, "mychart"), 0o755); err != nil {
			t.Fatalf("MkdirAll fake chart dir: %v", err)
		}
		return exec.Command("sh", "-c", "exit 0")
	}

	root, err := fetchAndExtractChart("oci://example.com/mychart")
	if err != nil {
		t.Fatalf("expected no error for stubbed OCI success, got %v", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("expected root directory from OCI success, got error: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected root to be a directory, got file")
	}
}
