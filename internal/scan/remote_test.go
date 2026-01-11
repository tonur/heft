package scan

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
