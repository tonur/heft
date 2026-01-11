package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

type helmIndexEntry struct {
	Version string   `yaml:"version"`
	URLs    []string `yaml:"urls"`
}

type helmIndex struct {
	Entries map[string][]helmIndexEntry `yaml:"entries"`
}

// resolveFromHelmIndex resolves a chart URL from a Helm repository index.
func resolveFromHelmIndex(repoURL, chartName, version string) (string, error) {
	if chartName == "" {
		return "", fmt.Errorf("chart name is empty")
	}

	index, err := loadHelmIndex(repoURL)
	if err != nil {
		return "", err
	}

	entries, ok := index.Entries[chartName]
	if !ok || len(entries) == 0 {
		return "", fmt.Errorf("chart %s not found in index", chartName)
	}

	entry, err := selectHelmChartEntry(entries, version)
	if err != nil {
		return "", err
	}

	if len(entry.URLs) == 0 {
		if version != "" {
			return "", fmt.Errorf("no URLs for chart %s version %s", chartName, version)
		}
		return "", fmt.Errorf("no URLs for chart %s best version", chartName)
	}

	return joinHelmURL(repoURL, entry.URLs[0])
}

func loadHelmIndex(repoURL string) (*helmIndex, error) {
	indexURL := strings.TrimRight(repoURL, "/") + "/index.yaml"

	response, err := http.Get(indexURL)
	if err != nil {
		return nil, fmt.Errorf("fetch index.yaml: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("index.yaml status %d: %s", response.StatusCode, string(body))
	}

	var index helmIndex
	if err := yaml.NewDecoder(response.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("parse index.yaml: %w", err)
	}

	return &index, nil
}

type versionedEntry struct {
	major int
	minor int
	patch int
	isPre bool
	idx   int
}

func parseVersionToEntry(version string, index int) versionedEntry {
	core := version
	pre := ""
	if dash := strings.Index(version, "-"); dash != -1 {
		core = version[:dash]
		pre = version[dash+1:]
	}
	parts := strings.Split(core, ".")
	entry := versionedEntry{idx: index, isPre: pre != ""}
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &entry.major)
	}
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &entry.minor)
	}
	if len(parts) > 2 {
		fmt.Sscanf(parts[2], "%d", &entry.patch)
	}
	return entry
}

func chooseBestVersion(entries []versionedEntry) versionedEntry {
	best := entries[0]
	for _, entry := range entries[1:] {
		if entry.major != best.major {
			if entry.major > best.major {
				best = entry
			}
			continue
		}
		if entry.minor != best.minor {
			if entry.minor > best.minor {
				best = entry
			}
			continue
		}
		if entry.patch != best.patch {
			if entry.patch > best.patch {
				best = entry
			}
			continue
		}
	}
	return best
}

func selectHelmChartEntry(entries []helmIndexEntry, version string) (helmIndexEntry, error) {
	if version != "" {
		for _, chartEntry := range entries {
			if chartEntry.Version == version {
				return chartEntry, nil
			}
		}
	}

	if len(entries) == 1 {
		return entries[0], nil
	}

	var stable, preReleases []versionedEntry
	for i, chartEntry := range entries {
		entry := parseVersionToEntry(chartEntry.Version, i)
		if entry.isPre {
			preReleases = append(preReleases, entry)
		} else {
			stable = append(stable, entry)
		}
	}

	var chosen versionedEntry
	if len(stable) > 0 {
		chosen = chooseBestVersion(stable)
	} else if len(preReleases) > 0 {
		chosen = chooseBestVersion(preReleases)
	} else {
		chosen = versionedEntry{idx: 0}
	}

	return entries[chosen.idx], nil
}

// joinHelmURL resolves a chart URL relative to a Helm repository URL.
func joinHelmURL(repositoryURL, chartURL string) (string, error) {
	repository, err := url.Parse(repositoryURL)
	if err != nil {
		return "", err
	}
	chart, err := url.Parse(chartURL)
	if err != nil {
		return "", err
	}
	return repository.ResolveReference(chart).String(), nil
}

// ociURLFromRepository constructs an OCI URL from a repository URL and chart name.
func ociURLFromRepository(repoURL, chartName string) (string, error) {
	if chartName == "" {
		return "", fmt.Errorf("chart name is empty")
	}

	trimmed := strings.TrimPrefix(repoURL, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "oci://")
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "", fmt.Errorf("invalid repository URL %q", repoURL)
	}

	return "oci://" + trimmed + "/" + chartName, nil
}
