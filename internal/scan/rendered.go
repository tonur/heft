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
	var conditions []string
	for _, d := range meta.Dependencies {
		if d.Condition != "" {
			conditions = append(conditions, d.Condition)
		}
	}
	return conditions
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
	for _, dependency := range meta.Dependencies {
		if dependency.Condition != "" && dependency.Name != "" {
			names = append(names, dependency.Name)
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
		arguments := []string{"template", "heft-scan"}
		// Preserve any user-specified Helm values flags.
		arguments = append(arguments, options.ValuesFiles...)
		arguments = append(arguments, options.Values...)

		// Treat ChartPath as the chart reference passed to helm template. This may be
		// a directory, a .tgz archive, an HTTP(S) URL, or an OCI reference.
		chartRef := options.ChartPath
		arguments = append(arguments, chartRef)

		if options.Verbose {
			fmt.Fprintf(os.Stderr, "heft: detectRendered: helm %s %s\n", helm, strings.Join(arguments, " "))
		}

		command := exec.Command(helm, arguments...)
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr

		if err := command.Run(); err != nil {
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

	documents := bytes.Split(out, []byte("---"))
	var images []ImageFinding

	for _, document := range documents {
		if len(bytes.TrimSpace(document)) == 0 {
			continue
		}

		var metadata map[string]any
		if err := yaml.Unmarshal(document, &metadata); err != nil {
			continue
		}

		kind, _ := metadata["kind"].(string)
		if kind == "" {
			continue
		}

		var podSpec any
		spec, _ := metadata["spec"].(map[string]any)
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "ReplicaSet", "ReplicationController":
			if template, ok := spec["template"].(map[string]any); ok {
				podSpec, _ = template["spec"]
			}
		case "Pod":
			podSpec = spec
		}

		podSpecMap, ok := podSpec.(map[string]any)
		if !ok {
			continue
		}

		extractFromContainerList := func(list any) {
			array, ok := list.([]any)
			if !ok {
				return
			}
			for _, item := range array {
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

		if c, ok := podSpecMap["containers"]; ok {
			extractFromContainerList(c)
		}
		if c, ok := podSpecMap["initContainers"]; ok {
			extractFromContainerList(c)
		}
		if c, ok := podSpecMap["ephemeralContainers"]; ok {
			extractFromContainerList(c)
		}
	}

	return images, nil
}
