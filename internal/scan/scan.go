package scan

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

var logWriter io.Writer = os.Stderr

type detectorConfig struct {
	name string
	run  func(Options) ([]ImageFinding, error)
}

func defaultDetectors() []detectorConfig {
	return []detectorConfig{
		{name: "rendered-manifest", run: detectRendered},
		{name: "static-chart", run: detectStatic},
		{name: "regex", run: detectRegex},
	}
}

// Scan runs the detectors in order of confidence and returns a ScanResult.
// It degrades gracefully: if higher-confidence detectors fail, lower
// confidence detectors are still attempted. An error is returned only if
// no detector produced any images.
func Scan(options Options) (*ScanResult, error) {
	if options.Verbose {
		fmt.Fprintf(logWriter, "heft: scan: chart=%q includeOptionalDeps=%v\n", options.ChartPath, options.IncludeOptionalDeps)
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
		if err := buildOptionalDependencies(options); err != nil {
			return nil, err
		}
	}

	var all []ImageFinding
	var warnings []error

	for _, detector := range defaultDetectors() {
		images, warn := runDetector(detector.name, options, detector.run)
		all = append(all, images...)
		if warn != nil {
			warnings = append(warnings, warn)
		}

		if detector.name == "rendered-manifest" && options.IncludeOptionalDeps {
			// When including optional dependencies, also scan each subchart under
			// charts/<name> if it exists locally. This complements the main
			// rendered-manifest scan of the parent chart and matches behavior like
			// running heft scan ./charts/<name> explicitly for each subchart.
			all = append(all, scanOptionalSubcharts(options)...)
		}
	}

	return finalizeScanResult(all, warnings, options.MinConfidence)
}

func buildOptionalDependencies(options Options) error {
	helm := options.HelmBin
	if helm == "" {
		helm = "helm"
	}
	dependencyCommand := exec.Command(helm, "dependency", "build", options.ChartPath)
	dependencyCommand.Env = os.Environ()
	var dependencyErrorOutput bytes.Buffer
	dependencyCommand.Stderr = &dependencyErrorOutput
	if err := dependencyCommand.Run(); err != nil {
		return fmt.Errorf("helm dependency build failed for %q: %w: %s", options.ChartPath, err, dependencyErrorOutput.String())
	}
	return nil
}

func runDetector(name string, options Options, detector func(Options) ([]ImageFinding, error)) ([]ImageFinding, error) {
	images, err := detector(options)
	if err != nil {
		wrapped := fmt.Errorf("%s detector failed: %w", name, err)
		if options.Verbose {
			fmt.Fprintf(logWriter, "heft: %s: chart=%q error=%v\n", name, options.ChartPath, err)
		}
		return nil, wrapped
	}

	if options.Verbose {
		fmt.Fprintf(logWriter, "heft: %s: chart=%q images=%d\n", name, options.ChartPath, len(images))
	}

	return images, nil
}

func finalizeScanResult(all []ImageFinding, warnings []error, min Confidence) (*ScanResult, error) {
	if len(all) == 0 {
		if len(warnings) > 0 {
			return nil, warnings[0]
		}
		return nil, fmt.Errorf("no images found by any detector")
	}

	for _, w := range warnings {
		fmt.Fprintln(logWriter, "heft: warning:", w)
	}

	deduped := dedupeImages(all)

	if min != "" && min != ConfidenceLow {
		filtered := deduped[:0]
		for _, image := range deduped {
			if image.Confidence == ConfidenceHigh || (min == ConfidenceMedium && image.Confidence == ConfidenceMedium) {
				filtered = append(filtered, image)
			}
		}
		deduped = filtered
	}

	return &ScanResult{Images: deduped}, nil
}
