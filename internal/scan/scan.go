package scan

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Scan runs the detectors in order of confidence and returns a ScanResult.
// It degrades gracefully: if higher-confidence detectors fail, lower
// confidence detectors are still attempted. An error is returned only if
// no detector produced any images.
func Scan(options Options) (*ScanResult, error) {
	if options.Verbose {
		fmt.Fprintf(os.Stderr, "heft: scan: chart=%q includeOptionalDeps=%v\n", options.ChartPath, options.IncludeOptionalDeps)
	}

	// Normalize remote chart references by downloading and extracting them
	// into a local directory so that all detectors can operate consistently.
	if isRemoteChartRef(options.ChartPath) {
		localRoot, err := fetchAndExtractChart(options.ChartPath)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch remote chart %q: %w", options.ChartPath, err)
		}
		options.ChartPath = localRoot
	}

	// If optional dependencies should be included, attempt to build chart
	// dependencies so that conditional subcharts (including remote/OCI ones)
	// are available locally before running rendered-manifest detection.
	if options.IncludeOptionalDeps {
		helm := options.HelmBin
		if helm == "" {
			helm = "helm"
		}
		depCmd := exec.Command(helm, "dependency", "build", options.ChartPath)
		depCmd.Env = os.Environ()
		var depStderr bytes.Buffer
		depCmd.Stderr = &depStderr
		if err := depCmd.Run(); err != nil {
			return nil, fmt.Errorf("helm dependency build failed for %q: %w: %s", options.ChartPath, err, depStderr.String())
		}
	}

	var all []ImageFinding
	var warnings []error

	if images, err := detectRendered(options); err != nil {
		warnings = append(warnings, fmt.Errorf("rendered-manifest detector failed: %w", err))
		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectRendered: chart=%q error=%v\n", options.ChartPath, err)
		}
	} else {
		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectRendered: chart=%q images=%d\\n", options.ChartPath, len(images))
		}
		all = append(all, images...)
	}

	// When including optional dependencies, also scan each subchart under
	// charts/<name> if it exists locally. This complements the main
	// rendered-manifest scan of the parent chart and matches behavior like
	// running heft scan ./charts/<name> explicitly for each subchart.
	if options.IncludeOptionalDeps {
		chartsDir := filepath.Join(options.ChartPath, "charts")
		entries, err := os.ReadDir(chartsDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				depChartPath := filepath.Join(chartsDir, e.Name())
				depOptions := options
				depOptions.ChartPath = depChartPath

				if options.Verbose {
					fmt.Fprintf(os.Stderr, "heft: scan: subchart=%q\n", depChartPath)
				}

				if images, err := detectRendered(depOptions); err == nil {
					if options.Verbose {
						fmt.Fprintf(os.Stderr, "heft: detectRendered: chart=%q images=%d\n", depChartPath, len(images))
					}
					all = append(all, images...)
				} else if options.Verbose {
					fmt.Fprintf(os.Stderr, "heft: detectRendered: chart=%q error=%v\n", depChartPath, err)
				}
				if images, err := detectStatic(depOptions); err == nil {
					if options.Verbose {
						fmt.Fprintf(os.Stderr, "heft: detectStatic: chart=%q images=%d\n", depChartPath, len(images))
					}
					all = append(all, images...)
				} else if options.Verbose {
					fmt.Fprintf(os.Stderr, "heft: detectStatic: chart=%q error=%v\n", depChartPath, err)
				}
				if images, err := detectRegex(depOptions); err == nil {
					if options.Verbose {
						fmt.Fprintf(os.Stderr, "heft: detectRegex: chart=%q images=%d\n", depChartPath, len(images))
					}
					all = append(all, images...)
				} else if options.Verbose {
					fmt.Fprintf(os.Stderr, "heft: detectRegex: chart=%q error=%v\n", depChartPath, err)
				}
			}
		}
	}

	if images, err := detectStatic(options); err != nil {
		warnings = append(warnings, fmt.Errorf("static-chart detector failed: %w", err))
		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectStatic: chart=%q error=%v\n", options.ChartPath, err)
		}
	} else {
		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectStatic: chart=%q images=%d\n", options.ChartPath, len(images))
		}
		all = append(all, images...)
	}

	if images, err := detectRegex(options); err != nil {
		warnings = append(warnings, fmt.Errorf("regex detector failed: %w", err))
		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectRegex: chart=%q error=%v\n", options.ChartPath, err)
		}
	} else {
		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectRegex: chart=%q images=%d\n", options.ChartPath, len(images))
		}
		all = append(all, images...)
	}

	if len(all) == 0 {
		// Surface the most relevant warning if we have one.
		if len(warnings) > 0 {
			return nil, warnings[0]
		}
		return nil, fmt.Errorf("no images found by any detector")
	}

	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "heft: warning:", w)
	}

	deduped := dedupeImages(all)

	// Filter by minimum confidence if requested.
	if options.MinConfidence != "" && options.MinConfidence != ConfidenceLow {
		filtered := deduped[:0]
		for _, image := range deduped {
			if image.Confidence == ConfidenceHigh || (options.MinConfidence == ConfidenceMedium && image.Confidence == ConfidenceMedium) {
				filtered = append(filtered, image)
			}
		}
		deduped = filtered
	}

	return &ScanResult{Images: deduped}, nil
}
