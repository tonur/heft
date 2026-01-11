package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDependencyHelpersUseConditions(t *testing.T) {
	dir, err := os.MkdirTemp("", "heft-rendered-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	chartYAML := `apiVersion: v2
name: testchart
dependencies:
  - name: dep1
    condition: dep1.enabled
  - name: dep2
    condition: dep2.enabled
  - name: dep3
`
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYAML), 0o644); err != nil {
		t.Fatalf("WriteFile Chart.yaml: %v", err)
	}

	conditions := loadDependencyConditions(dir)
	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %v", conditions)
	}

	names := dependencyNamesWithConditions(dir)
	if len(names) != 2 {
		t.Fatalf("expected 2 dependency names with conditions, got %v", names)
	}
	if names[0] != "dep1" || names[1] != "dep2" {
		t.Fatalf("unexpected dependency names: %v", names)
	}
}
