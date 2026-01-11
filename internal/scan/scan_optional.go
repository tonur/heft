package scan

import (
	"fmt"
	"os"
	"path/filepath"
)

func scanOptionalSubcharts(options Options) []ImageFinding {
	var all []ImageFinding
	chartsDir := filepath.Join(options.ChartPath, "charts")
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		depChartPath := filepath.Join(chartsDir, entry.Name())
		depOptions := options
		depOptions.ChartPath = depChartPath

		if options.Verbose {
			fmt.Fprintf(logWriter, "heft: scan: subchart=%q\n", depChartPath)
		}

		if images, err := detectRendered(depOptions); err == nil {
			if options.Verbose {
				fmt.Fprintf(logWriter, "heft: detectRendered: chart=%q images=%d\n", depChartPath, len(images))
			}
			all = append(all, images...)
		} else if options.Verbose {
			fmt.Fprintf(logWriter, "heft: detectRendered: chart=%q error=%v\n", depChartPath, err)
		}

		if images, err := detectStatic(depOptions); err == nil {
			if options.Verbose {
				fmt.Fprintf(logWriter, "heft: detectStatic: chart=%q images=%d\n", depChartPath, len(images))
			}
			all = append(all, images...)
		} else if options.Verbose {
			fmt.Fprintf(logWriter, "heft: detectStatic: chart=%q error=%v\n", depChartPath, err)
		}

		if images, err := detectRegex(depOptions); err == nil {
			if options.Verbose {
				fmt.Fprintf(logWriter, "heft: detectRegex: chart=%q images=%d\n", depChartPath, len(images))
			}
			all = append(all, images...)
		} else if options.Verbose {
			fmt.Fprintf(logWriter, "heft: detectRegex: chart=%q error=%v\n", depChartPath, err)
		}
	}

	return all
}
