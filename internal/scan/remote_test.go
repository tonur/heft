package scan

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// helper to create a small .tar.gz in a temp file.
func createTestTarGz(t *testing.T, files map[string]string) string {
	t.Helper()

	f, err := os.CreateTemp("", "heft-remote-*.tar.gz")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatalf("Write body: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file Close: %v", err)
	}

	return f.Name()
}

func TestDownloadFileAndExtractTarGz(t *testing.T) {
	tarPath := createTestTarGz(t, map[string]string{"file.txt": "hello"})
	defer os.RemoveAll(tarPath)

	// Serve the tar.gz via HTTP.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(tarPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	// downloadFile should save the content locally.
	tmpFile, err := os.CreateTemp("", "heft-download-*.tar.gz")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmpFile.Close()

	if err := downloadFile(srv.URL, tmpFile.Name()); err != nil {
		t.Fatalf("downloadFile error: %v", err)
	}

	// extractTarGz should extract the file into a directory.
	outDir, err := os.MkdirTemp("", "heft-extract-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(outDir)

	root, err := extractTarGz(tmpFile.Name(), outDir)
	if err != nil {
		t.Fatalf("extractTarGz error: %v", err)
	}

	content, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("expected extracted file: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected extracted content: %q", string(content))
	}
}

func TestFetchAndExtractChart(t *testing.T) {
	tarPath := createTestTarGz(t, map[string]string{"Chart.yaml": "apiVersion: v2"})
	defer os.RemoveAll(tarPath)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(tarPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	chartPath, err := fetchAndExtractChart(srv.URL)
	if err != nil {
		t.Fatalf("fetchAndExtractChart error: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(chartPath))

	// Expect Chart.yaml exists at the path returned.
	if _, err := os.Stat(chartPath); err != nil {
		t.Fatalf("expected Chart.yaml at %s: %v", chartPath, err)
	}
}

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

func TestExtractTarGzWithInvalidGzip(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.tgz")
	if err := os.WriteFile(badPath, []byte("not-a-gzip"), 0o644); err != nil {
		t.Fatalf("WriteFile bad.tgz: %v", err)
	}

	if _, err := extractTarGz(badPath, dir); err == nil {
		t.Fatalf("expected error for invalid gzip input, got nil")
	}
}

func TestExtractTarGzWithEmptyArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.tgz")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create empty.tgz: %v", err)
	}
	gz := gzip.NewWriter(f)
	if err := gz.Close(); err != nil {
		f.Close()
		t.Fatalf("Close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close file: %v", err)
	}

	if _, err := extractTarGz(path, dir); err == nil {
		t.Fatalf("expected error for archive with no root directory, got nil")
	}
}

func TestExtractTarGzWithSingleFileSetsRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single-file.tgz")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create single-file.tgz: %v", err)
	}
	gz := gzip.NewWriter(f)
	tr := tar.NewWriter(gz)

	// Write a single regular file at top-level.
	hdr := &tar.Header{
		Name:     "mychart/Chart.yaml",
		Mode:     0o644,
		Size:     int64(len("content")),
		Typeflag: tar.TypeReg,
	}
	if err := tr.WriteHeader(hdr); err != nil {
		tr.Close()
		gz.Close()
		f.Close()
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tr.Write([]byte("content")); err != nil {
		tr.Close()
		gz.Close()
		f.Close()
		t.Fatalf("Write: %v", err)
	}

	if err := tr.Close(); err != nil {
		gz.Close()
		f.Close()
		t.Fatalf("Close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		f.Close()
		t.Fatalf("Close gzip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close file: %v", err)
	}

	root, err := extractTarGz(path, dir)
	if err != nil {
		t.Fatalf("extractTarGz error: %v", err)
	}
	expectedRoot := filepath.Join(dir, "mychart")
	if root != expectedRoot {
		t.Fatalf("unexpected root: got %q, want %q", root, expectedRoot)
	}

	// Ensure the file was extracted.
	if _, err := os.Stat(filepath.Join(expectedRoot, "Chart.yaml")); err != nil {
		t.Fatalf("expected extracted file Chart.yaml: %v", err)
	}
}

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
