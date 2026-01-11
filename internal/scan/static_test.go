package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectStaticEmptyChartPath(t *testing.T) {
	if _, err := detectStatic(Options{ChartPath: ""}); err == nil {
		t.Fatalf("expected error for empty chart path, got nil")
	}
}

func TestDetectStaticCollectsImagesFromVariousPatterns(t *testing.T) {
	root := t.TempDir()
	chartDir := filepath.Join(root, "chart")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatalf("MkdirAll chartDir: %v", err)
	}

	valuesPath := filepath.Join(chartDir, "values.yaml")
	content := `image: nginx:1.0
---
image:
  repository: ghcr.io/example/app
  tag: v2
---
image: "{{ .Values.image.repository }}"
---
containers:
  - image: alpine:3.18
`
	if err := os.WriteFile(valuesPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile values.yaml: %v", err)
	}

	results, err := detectStatic(Options{ChartPath: chartDir})
	if err != nil {
		t.Fatalf("detectStatic error: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected some static images, got 0")
	}

	var sawNginx, sawGhcr, sawAlpine, sawTemplate bool
	for _, r := range results {
		switch r.Name {
		case "nginx:1.0":
			sawNginx = true
		case "ghcr.io/example/app:v2":
			sawGhcr = true
		case "alpine:3.18":
			sawAlpine = true
		case "{{ .Values.image.repository }}":
			sawTemplate = true
		}
	}

	if !sawNginx || !sawGhcr || !sawAlpine {
		t.Fatalf("missing expected images: nginx=%v ghcr=%v alpine=%v", sawNginx, sawGhcr, sawAlpine)
	}
	if sawTemplate {
		t.Fatalf("templated image should not be included in results")
	}
}
