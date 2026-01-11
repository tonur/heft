package scan

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

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
