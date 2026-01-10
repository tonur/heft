package scan

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// detectStatic performs a best-effort static analysis of chart YAML files
// when rendering is not available or incomplete.
func detectStatic(opts Options) ([]ImageFinding, error) {
	var results []ImageFinding

	root := opts.ChartPath
	if root == "" {
		return nil, fmt.Errorf("chart path is empty")
	}

	// Walk the chart directory and inspect YAML files.
	// We intentionally avoid following symlinks or special file types.
	//
	// Note that this is a heuristic detector and may miss images that are
	// constructed dynamically via templates.
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		lower := strings.ToLower(path)
		if !(strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")) {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			// Best-effort: skip unreadable files.
			return nil
		}

		// Split possible multi-doc YAML
		for _, doc := range bytes.Split(data, []byte("---")) {
			if len(bytes.TrimSpace(doc)) == 0 {
				continue
			}
			var m map[string]any
			if err := yaml.Unmarshal(doc, &m); err != nil {
				continue
			}
			collectStaticImages(m, path, &results)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

// collectStaticImages recursively walks YAML structures looking for common
// image patterns.
func collectStaticImages(node any, file string, results *[]ImageFinding) {
	switch value := node.(type) {
	case map[string]any:
		// Check direct "image" key
		if imageValue, ok := value["image"]; ok {
			if name, ok := imageValue.(string); ok && name != "" && !strings.Contains(name, "{{") {
				*results = append(*results, ImageFinding{
					Name:       name,
					Confidence: ConfidenceMedium,
					Source:     SourceKind("static-yaml"),
					File:       file,
				})
			}
			if m, ok := imageValue.(map[string]any); ok {
				repo, _ := m["repository"].(string)
				tag, _ := m["tag"].(string)
				if repo != "" && !strings.Contains(repo, "{{") {
					name := repo
					if tag != "" && !strings.Contains(tag, "{{") {
						name = fmt.Sprintf("%s:%s", repo, tag)
					}
					*results = append(*results, ImageFinding{
						Name:       name,
						Confidence: ConfidenceMedium,
						Source:     SourceKind("static-yaml"),
						File:       file,
					})
				}
			}
		}
		// Recurse into children
		for _, child := range value {
			collectStaticImages(child, file, results)
		}
	case []any:
		for _, item := range value {
			collectStaticImages(item, file, results)
		}
	}
}
