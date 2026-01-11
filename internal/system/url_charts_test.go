//go:build e2e

package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type chartFixture struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type commandFixture struct {
	Name           string          `yaml:"name"`
	Arguments      []string        `yaml:"arguments"`
	ExpectedImages []expectedImage `yaml:"expectedImages"`
}

func loadChartFixture(dir string) (*chartFixture, error) {
	data, err := os.ReadFile(filepath.Join(dir, "chart_metadata.yaml"))
	if err != nil {
		return nil, err
	}
	var chartFixture chartFixture
	if err := yaml.Unmarshal(data, &chartFixture); err != nil {
		return nil, err
	}
	if chartFixture.Name == "" {
		chartFixture.Name = filepath.Base(dir)
	}
	if chartFixture.URL == "" {
		return nil, fmt.Errorf("chart url must be set for %s", chartFixture.Name)
	}
	return &chartFixture, nil
}

func runChartCommands(t *testing.T, binPath string, chart *chartFixture, commandsDirectory string) {
	entries, err := os.ReadDir(commandsDirectory)
	if err != nil {
		t.Fatalf("failed to read commands dir %s: %v", commandsDirectory, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(commandsDirectory, entry.Name())

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read command fixture %s: %v", path, err)
		}

		var commandFixture commandFixture
		if err := yaml.Unmarshal(data, &commandFixture); err != nil {
			t.Fatalf("failed to unmarshal command fixture %s: %v", path, err)
		}

		if len(commandFixture.Arguments) == 0 {
			t.Fatalf("command fixture %s has no arguments", path)
		}
		if len(commandFixture.ExpectedImages) == 0 {
			t.Fatalf("command fixture %s has no expectedImages", path)
		}

		arguments := make([]string, len(commandFixture.Arguments))
		for i, argument := range commandFixture.Arguments {
			if argument == "${CHART_URL}" {
				arguments[i] = chart.URL
			} else {
				arguments[i] = argument
			}
		}

		name := commandFixture.Name
		if name == "" {
			name = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}

		t.Run(name, func(t *testing.T) {
			out := runHeftScan(t, binPath, arguments...)

			var parsed scanOutput
			if err := yaml.Unmarshal(out, &parsed); err != nil {
				t.Fatalf("failed to parse YAML output for %s/%s: %v\noutput:\n%s", chart.Name, name, err, out)
			}

			for _, expected := range commandFixture.ExpectedImages {
				if expected.Image == "" {
					continue
				}
				found := false
				for _, image := range parsed.Images {
					if image.Name == expected.Image && image.Confidence == expected.Confidence && image.Source == expected.Source {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected image not found for %s/%s: image=%q confidence=%q source=%q\nparsed=%+v", chart.Name, name, expected.Image, expected.Confidence, expected.Source, parsed)
				}
			}
		})
	}
}

func TestHeftScanChartsFromFixtures(t *testing.T) {
	binPath := buildHeftBinary(t)
	repositoryRoot := repositoryRootDirectory(t)
	chartsRoot := filepath.Join(repositoryRoot, "internal", "system", "testdata", "charts")

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
