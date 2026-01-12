package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type artifactHubSearchResponse struct {
	Charts []artifactHubChart `json:"packages"`
}

type artifactHubChart struct {
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name"`
	Version        string `json:"version"`
	ContentURL     string `json:"content_url"`
	Repository     struct {
		Kind int    `json:"kind"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"repository"`
}

var artifactHubBaseURL = "https://artifacthub.io"

// fetchArtifactHubCharts queries Artifact Hub for helm charts with the
// given pagination and sort order.
// fetchArtifactHubChartsFunction is a variable to allow tests to stub the
// Artifact Hub search.
var fetchArtifactHubChartsFunction = fetchArtifactHubCharts

func fetchArtifactHubCharts(limit, offset int, sort string) ([]artifactHubChart, error) {
	url := fmt.Sprintf("%s/api/v1/packages/search?kind=0&limit=%d&offset=%d&sort=%s", strings.TrimRight(artifactHubBaseURL, "/"), limit, offset, sort)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("artifacthub search: status %d: %s", response.StatusCode, string(body))
	}

	var searchResponse artifactHubSearchResponse
	if err := json.NewDecoder(response.Body).Decode(&searchResponse); err != nil {
		return nil, err
	}

	return searchResponse.Charts, nil
}
