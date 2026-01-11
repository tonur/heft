package scan

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func isRemoteChartRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "oci://")
}

// helmPullCommand is a variable to allow tests to stub the helm pull
// invocation used for OCI chart references.
var helmPullCommand = func(ref, tmpDir string) *exec.Cmd {
	helm := "helm"
	return exec.Command(helm, "pull", ref, "--untar", "--untardir", tmpDir)
}

func fetchAndExtractChart(ref string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "heft-chart-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		// Download the chart archive and extract it.
		tgzPath := filepath.Join(tmpDir, "chart.tgz")
		if err := downloadFile(ref, tgzPath); err != nil {
			return "", fmt.Errorf("download chart: %w", err)
		}

		rootDir, err := extractTarGz(tgzPath, tmpDir)
		if err != nil {
			return "", fmt.Errorf("extract chart: %w", err)
		}
		return rootDir, nil
	}

	if strings.HasPrefix(ref, "oci://") {
		command := helmPullCommand(ref, tmpDir)
		command.Env = os.Environ()
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Run(); err != nil {
			return "", fmt.Errorf("helm pull failed: %w: %s", err, stderr.String())
		}

		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			return "", fmt.Errorf("read pulled chart dir: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				return filepath.Join(tmpDir, e.Name()), nil
			}
		}
		return "", fmt.Errorf("no chart directory found after helm pull")
	}

	return "", fmt.Errorf("unsupported remote chart ref: %q", ref)
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractTarGz(tgzPath, destDir string) (string, error) {
	f, err := os.Open(tgzPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var root string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Normalize header name to avoid leading "./" differences and
		// identify the top-level chart directory from the first path segment.
		name := hdr.Name
		if after, ok := strings.CutPrefix(name, "./"); ok {
			name = after
		}
		if name == "" {
			continue
		}

		// Set root to the first path segment we see; this works even if the
		// archive does not contain an explicit directory header for the root.
		if root == "" {
			parts := strings.SplitN(name, string(os.PathSeparator), 2)
			root = filepath.Join(destDir, parts[0])
		}

		target := filepath.Join(destDir, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			out, err := os.Create(target)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
		}
	}

	if root == "" {
		return "", fmt.Errorf("no root directory found in chart archive")
	}
	return root, nil
}
