package scan

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// minimal structs for parsing Chart.yaml dependency conditions
type chartDependency struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"`
}

type chartMetadata struct {
	Dependencies []chartDependency `yaml:"dependencies"`
}

func loadDependencyConditions(chartPath string) []string {
	chartFile := filepath.Join(chartPath, "Chart.yaml")
	data, err := os.ReadFile(chartFile)
	if err != nil {
		return nil
	}
	var meta chartMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil
	}
	var conds []string
	for _, d := range meta.Dependencies {
		if d.Condition != "" {
			conds = append(conds, d.Condition)
		}
	}
	return conds
}

func dependencyNamesWithConditions(chartPath string) []string {
	chartFile := filepath.Join(chartPath, "Chart.yaml")
	data, err := os.ReadFile(chartFile)
	if err != nil {
		return nil
	}
	var meta chartMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil
	}
	var names []string
	for _, d := range meta.Dependencies {
		if d.Condition != "" && d.Name != "" {
			names = append(names, d.Name)
		}
	}
	return names
}

// detectRendered invokes `helm template` and extracts images from rendered YAML.
func detectRendered(options Options) ([]ImageFinding, error) {
	helm := options.HelmBin
	if helm == "" {
		helm = "helm"
	}

	template := func() ([]byte, error) {
		args := []string{"template", "heft-scan"}
		// Preserve any user-specified Helm values flags.
		args = append(args, options.ValuesFiles...)
		args = append(args, options.Values...)

		// Treat ChartPath as the chart reference passed to helm template. This may be
		// a directory, a .tgz archive, an HTTP(S) URL, or an OCI reference.
		chartRef := options.ChartPath
		args = append(args, chartRef)

		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectRendered: helm %s %s\n", helm, strings.Join(args, " "))
		}

		cmd := exec.Command(helm, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			if options.Verbose {
				fmt.Fprintf(os.Stderr, "heft: detectRendered: helm error: %s\n", stderr.String())
			}
			return nil, fmt.Errorf("helm template failed: %w: %s", err, stderr.String())
		}
		return stdout.Bytes(), nil
	}

	out, err := template()
	if err != nil {
		// If auto dependency builds are disabled, just return the error.
		if options.DisableHelmDeps {
			return nil, err
		}
		// If this looks like a missing dependency error on a local chart dir,
		// try a best-effort "helm dependency build" once and retry template.
		if !isRemoteChartRef(options.ChartPath) && (strings.Contains(err.Error(), "helm dependency build") || strings.Contains(err.Error(), "missing in charts/ directory")) {
			depCmd := exec.Command(helm, "dependency", "build", options.ChartPath)
			var depStderr bytes.Buffer
			depCmd.Stderr = &depStderr
			if depErr := depCmd.Run(); depErr == nil {
				if out2, tplErr := template(); tplErr == nil {
					out = out2
					err = nil
				}
			}
		}
		if err != nil {
			return nil, err
		}
	}

	docs := bytes.Split(out, []byte("---"))
	var images []ImageFinding

	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		var m map[string]any
		if err := yaml.Unmarshal(doc, &m); err != nil {
			continue
		}

		kind, _ := m["kind"].(string)
		if kind == "" {
			continue
		}

		var podSpec any
		spec, _ := m["spec"].(map[string]any)
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "ReplicaSet", "ReplicationController":
			if tmpl, ok := spec["template"].(map[string]any); ok {
				podSpec, _ = tmpl["spec"]
			}
		case "Pod":
			podSpec = spec
		}

		ps, ok := podSpec.(map[string]any)
		if !ok {
			continue
		}

		extractFromContainerList := func(list any) {
			arr, ok := list.([]any)
			if !ok {
				return
			}
			for _, item := range arr {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if image, ok := m["image"].(string); ok && image != "" {
					images = append(images, ImageFinding{
						Name:       image,
						Confidence: ConfidenceHigh,
						Source:     SourceRendered,
					})
				}
			}
		}

		if c, ok := ps["containers"]; ok {
			extractFromContainerList(c)
		}
		if c, ok := ps["initContainers"]; ok {
			extractFromContainerList(c)
		}
		if c, ok := ps["ephemeralContainers"]; ok {
			extractFromContainerList(c)
		}
	}

	return images, nil
}
