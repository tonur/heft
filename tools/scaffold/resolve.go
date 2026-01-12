package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

// helmIndex represents the minimal structure we need from a Helm index.yaml.
// entries:
//
//	<name>:
//	  - version: "1.2.3"
//	    urls: ["chart-1.2.3.tgz"]
type helmIndex struct {
	Entries map[string][]struct {
		Version string   `yaml:"version"`
		URLs    []string `yaml:"urls"`
	} `yaml:"entries"`
}

// versionedEntry is a parsed semantic version with original slice index.
type versionedEntry struct {
	major int
	minor int
	patch int
	isPre bool
	idx   int
}

// loadHelmIndex fetches and parses a Helm index.yaml from the given URL.
func loadHelmIndex(repoURL string) (*helmIndex, error) {
	indexURL := strings.TrimRight(repoURL, "/") + "/index.yaml"

	response, err := http.Get(indexURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, response.Body)
		return nil, fmt.Errorf("index.yaml status %d: %s", response.StatusCode, buf.String())
	}

	var index helmIndex
	if err := yaml.NewDecoder(response.Body).Decode(&index); err != nil {
		return nil, err
	}
	return &index, nil
}

// joinHelmURL joins a repository URL and a chart URL from index.yaml.
func joinHelmURL(repository, chart string) (string, error) {
	repoURL, err := url.Parse(repository)
	if err != nil {
		return "", err
	}
	chartURL, err := url.Parse(chart)
	if err != nil {
		return "", err
	}
	return repoURL.ResolveReference(chartURL).String(), nil
}

// chooseBestVersion picks the highest version, preferring stable over pre-release.
func chooseBestVersion(entries []versionedEntry) versionedEntry {
	if len(entries) == 0 {
		return versionedEntry{}
	}
	best := entries[0]
	for _, e := range entries[1:] {
		// If numeric parts are equal, prefer stable over pre-release.
		if e.major == best.major && e.minor == best.minor && e.patch == best.patch {
			if best.isPre && !e.isPre {
				best = e
			}
			continue
		}

		// Compare numeric parts.
		numericBetter := e.major > best.major ||
			(e.major == best.major && e.minor > best.minor) ||
			(e.major == best.major && e.minor == best.minor && e.patch > best.patch)

		if !numericBetter {
			continue
		}

		// If the candidate is pre-release and the current best is stable,
		// keep the stable best, even if the candidate has a higher numeric
		// version.
		if e.isPre && !best.isPre {
			continue
		}

		best = e
	}
	return best
}

// resolveFromHelmIndex looks up a chart in index.yaml and returns a full URL
// using the repository base.
func resolveFromHelmIndex(repositoryURL, chartName, version string) (string, error) {
	if strings.TrimSpace(chartName) == "" {
		return "", fmt.Errorf("chart name is empty")
	}

	index, err := loadHelmIndex(repositoryURL)
	if err != nil {
		return "", err
	}

	entries, ok := index.Entries[chartName]
	if !ok || len(entries) == 0 {
		return "", fmt.Errorf("chart %s not found in index", chartName)
	}

	// If a specific version is requested, find it.
	if version != "" {
		for _, entry := range entries {
			entryVersion := strings.ToLower(entry.Version)
			if entryVersion == version || entryVersion == "v"+version {
				if len(entry.URLs) == 0 {
					return "", fmt.Errorf("no URLs for chart %s version %s", chartName, version)
				}
				return joinHelmURL(repositoryURL, entry.URLs[0])
			}
		}
		return "", fmt.Errorf("chart %s version %s not found in index", chartName, version)
	}

	// Otherwise choose the best version by numeric components.
	var vers []versionedEntry
	for i, e := range entries {
		// Very small parser: split on '.', ignore non-numeric.
		parts := strings.SplitN(e.Version, "-", 2)
		core := parts[0]
		pre := len(parts) > 1
		numbers := strings.Split(core, ".")
		if len(numbers) != 3 {
			continue
		}
		var v versionedEntry
		fmt.Sscanf(core, "%d.%d.%d", &v.major, &v.minor, &v.patch)
		v.isPre = pre
		v.idx = i
		vers = append(vers, v)
	}

	if len(vers) == 0 {
		return "", fmt.Errorf("no URLs for chart %s best version", chartName)
	}

	best := chooseBestVersion(vers)
	chosen := entries[best.idx]
	if len(chosen.URLs) == 0 {
		return "", fmt.Errorf("no URLs for chart %s best version", chartName)
	}

	return joinHelmURL(repositoryURL, chosen.URLs[0])
}

// ociURLFromRepository constructs an OCI URL from an Artifact Hub repository.
// It accepts both oci:// and https:// style URLs and normalizes to oci://.
func ociURLFromRepository(repoURL, chartName string) (string, error) {
	repoURL = strings.TrimSpace(repoURL)
	chartName = strings.TrimSpace(chartName)

	if chartName == "" {
		return "", fmt.Errorf("chart name is empty")
	}
	if repoURL == "" {
		return "", fmt.Errorf("repository URL is empty")
	}

	// Native OCI URL: strip prefix and trailing slash.
	if strings.HasPrefix(repoURL, "oci://") {
		trimmed := strings.TrimPrefix(repoURL, "oci://")
		trimmed = strings.TrimRight(trimmed, "/")
		return fmt.Sprintf("oci://%s/%s", trimmed, chartName), nil
	}

	// HTTPS/HTTP registry URL: normalize host+path.
	if strings.HasPrefix(repoURL, "https://") || strings.HasPrefix(repoURL, "http://") {
		u, err := url.Parse(repoURL)
		if err != nil {
			return "", err
		}
		hostAndPath := strings.TrimRight(u.Host+u.Path, "/")
		return fmt.Sprintf("oci://%s/%s", hostAndPath, chartName), nil
	}

	return "", fmt.Errorf("repository %s is not an OCI repository", repoURL)
}
