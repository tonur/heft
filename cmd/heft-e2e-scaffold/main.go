package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var exitFunc = os.Exit

func main() {
	maxCharts := flag.Int("max-charts", 10, "maximum number of new charts to scaffold")
	minConfidence := flag.String("min-confidence", "high", "minimum confidence passed to heft scan")
	artifactHubSort := flag.String("sort", "stars", "sort field for Artifact Hub search (e.g. stars, score)")
	flag.Parse()

	if err := run(*maxCharts, *minConfidence, *artifactHubSort); err != nil {
		fmt.Fprintf(os.Stderr, "heft-e2e-scaffold error: %v\n", err)
		exitFunc(1)
	}
}

// run orchestrates fetching charts from Artifact Hub and scaffolding
// test fixtures for each chart.
func run(maxCharts int, minConfidence, sort string) error {
	repositoryRoot, err := repositoryRoot()
	if err != nil {
		return err
	}

	heftPath, err := ensureHeftBinary(repositoryRoot)
	if err != nil {
		return fmt.Errorf("ensure heft binary: %w", err)
	}

	fmt.Printf("Using heft binary at %s\n", heftPath)

	chartsRoot := filepath.Join(repositoryRoot, "internal", "system", "testdata", "charts")
	if err := os.MkdirAll(chartsRoot, 0o755); err != nil {
		return fmt.Errorf("create chartsRoot: %w", err)
	}

	newCharts := 0
	offset := 0
	pageLimit := 60

	for newCharts < maxCharts {
		charts, err := fetchArtifactHubCharts(pageLimit, offset, sort)
		if err != nil {
			return fmt.Errorf("fetch Artifact Hub charts: %w", err)
		}
		if len(charts) == 0 {
			break
		}

		for _, chart := range charts {
			if newCharts >= maxCharts {
				break
			}

			name := chart.NormalizedName
			if name == "" {
				name = chart.Name
			}
			if name == "" {
				continue
			}

			chartDir := filepath.Join(chartsRoot, name)
			metadataPath := filepath.Join(chartDir, "chart_metadata.yaml")
			if _, err := os.Stat(metadataPath); err == nil {
				// already scaffolded
				continue
			}

			chartURL, err := resolveChartURL(chart)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Skipping %s: cannot resolve chart ref: %v\n", name, err)
				continue
			}

			if err := scaffoldChart(chartDir, metadataPath, &chart, chartURL, heftPath, minConfidence); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to scaffold chart %s: %v\n", name, err)
				continue
			}
			newCharts++
		}

		offset += pageLimit
		// Be polite to Artifact Hub.
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("Scaffolded %d new chart test cases\n", newCharts)
	return nil
}
