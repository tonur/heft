package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var exitFunction = os.Exit

func main() {
	maxCharts := flag.Int("max-charts", 10, "maximum number of new charts to scaffold")
	minConfidence := flag.String("min-confidence", "high", "minimum confidence passed to heft scan")
	artifactHubSort := flag.String("sort", "stars", "sort field for Artifact Hub search (e.g. stars, score)")
	update := flag.Bool("update", false, "update metadata for existing charts and warn on drift")
	flag.Parse()

	if err := runFunction(*maxCharts, *minConfidence, *artifactHubSort, *update); err != nil {
		fmt.Fprintf(os.Stderr, "heft-e2e-scaffold error: %v\n", err)
		exitFunction(1)
	}
}

// runFunction is a function variable to allow tests to stub run.
var runFunction = run

// run orchestrates fetching charts from Artifact Hub and scaffolding
// test fixtures for each chart.
func run(maxCharts int, minConfidence, sort string, update bool) error {
	repositoryRoot, err := repositoryRootFunction()
	if err != nil {
		return err
	}

	heftPath, err := ensureHeftBinaryFunction(repositoryRoot)
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

	for newCharts < maxCharts || update {
		charts, err := fetchArtifactHubChartsFunction(pageLimit, offset, sort)
		if err != nil {
			return fmt.Errorf("fetch Artifact Hub charts: %w", err)
		}
		if len(charts) == 0 {
			break
		}

		for _, chart := range charts {
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
				if !update {
					// already scaffolded in normal mode
					continue
				}

				chartURL, err := resolveChartURL(chart)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Skipping %s: cannot resolve chart ref: %v\n", name, err)
					continue
				}

				if err := updateChartMetadataAndCheckDrift(chartDir, metadataPath, &chart, chartURL, heftPath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to update chart %s: %v\n", name, err)
				}
				continue
			}

			if newCharts >= maxCharts {
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

		if !update && newCharts >= maxCharts {
			break
		}
	}

	fmt.Printf("Scaffolded %d new chart test cases\n", newCharts)
	return nil
}
