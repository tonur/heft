//go:build e2e

package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHeftScanChartsFromFixtures(t *testing.T) {
	binPath := buildHeftBinary(t)
	repositoryRoot := repositoryRootDirectory(t)
	chartsRoot := filepath.Join(repositoryRoot, "testcases")

	entries, err := os.ReadDir(chartsRoot)
	if err != nil {
		t.Fatalf("failed to read charts testdata dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		chartDirectory := filepath.Join(chartsRoot, entry.Name())

		chart, err := loadChartFixture(chartDirectory)
		if err != nil {
			t.Fatalf("failed to load chart fixture from %s: %v", chartDirectory, err)
		}

		commandsDirectory := filepath.Join(chartDirectory, "commands")
		t.Run(chart.Name, func(t *testing.T) {
			runChartCommands(t, binPath, chart, commandsDirectory)
		})
	}
}
