package scan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectRegex performs a heuristic scan for image-like patterns in chart files.
func detectRegex(opts Options) ([]ImageFinding, error) {
	var results []ImageFinding

	root := opts.ChartPath
	if root == "" {
		return nil, fmt.Errorf("chart path is empty")
	}

	// Basic regex for image:tag or image@sha256:..., kept intentionally
	// conservative to reduce false positives.
	pattern := regexp.MustCompile(`(?m)
(?:[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?(?::\d+)?/)?
[a-z0-9]+(?:[._-][a-z0-9]+)*
(?:
  :\w[\w.-]{0,127}
 |@sha256:[A-Fa-f0-9]{64}
)?
`)

	// Walk the chart directory and scan text files for image-like substrings.
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Skip obvious test/snapshot files to reduce noise.
		lowerPath := strings.ToLower(path)
		if strings.Contains(lowerPath, "/tests/") || strings.Contains(lowerPath, "__snapshot__") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			matches := pattern.FindAllString(line, -1)
			for _, m := range matches {
				// Drop obviously junky tags like ":." which usually
				// come from templating placeholders or partial strings.
				if strings.HasSuffix(m, "=.") || strings.HasSuffix(m, ":.") {
					continue
				}
				results = append(results, ImageFinding{
					Name:       m,
					Confidence: ConfidenceLow,
					Source:     SourceKind("regex-scan"),
					File:       path,
					Line:       i + 1,
				})
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}
