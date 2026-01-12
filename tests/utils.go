//go:build e2e

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type expectedImage struct {
	Image      string
	Confidence string
	Source     string
}

type scanImage struct {
	Name       string `yaml:"name"`
	Confidence string `yaml:"confidence"`
	Source     string `yaml:"source"`
}

type scanOutput struct {
	Images []scanImage `yaml:"images"`
}

type chartFixture struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type commandFixture struct {
	Name           string          `yaml:"name"`
	Arguments      []string        `yaml:"arguments"`
	ExpectedImages []expectedImage `yaml:"expectedImages"`
}

func buildHeftBinary(t *testing.T) string {
	t.Helper()

	// Ensure helm is available since heft relies on it.
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not available in PATH; skipping e2e test")
	}

	temporaryDirectory := t.TempDir()
	binaryPath := filepath.Join(temporaryDirectory, "heft")

	repositoryRoot := repositoryRootDirectory(t)

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/heft")
	build.Env = os.Environ()
	build.Dir = repositoryRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build heft: %v\n%s", err, out)
	}

	return binaryPath
}

func repositoryRootDirectory(t *testing.T) string {
	t.Helper()

	repositoryRoot, err := filepath.Abs(filepath.Join(".."))
	if err != nil {
		t.Fatalf("failed to resolve repository root: %v", err)
	}
	return repositoryRoot
}

func runHeftScan(t *testing.T, binPath string, arguments ...string) []byte {
	t.Helper()

	temporaryDir := t.TempDir()
	command := exec.Command(binPath, arguments...)
	command.Env = os.Environ()
	command.Dir = temporaryDir

	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("heft scan failed: %v\noutput:\n%s", err, out)
	}

	return out
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
