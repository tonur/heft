package main

import "strings"

// firstNonEmpty returns the first non-empty, non-whitespace string from
// the provided values.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// normalizeImageName normalizes container image names so that registry
// hosts are explicit and tags are present when omitted.
func normalizeImageName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}

	baseForHost := name
	if at := strings.Index(baseForHost, "@"); at != -1 {
		baseForHost = baseForHost[:at]
	}
	if colon := strings.Index(baseForHost, ":"); colon != -1 {
		baseForHost = baseForHost[:colon]
	}

	first := baseForHost
	if slash := strings.Index(first, "/"); slash != -1 {
		first = first[:slash]
	}

	if strings.Contains(first, ".") {
		return name
	}

	if strings.Contains(baseForHost, "/") {
		base := name
		if !strings.Contains(base, ":") && !strings.Contains(base, "@") {
			base = base + ":latest"
		}
		return "docker.io/" + base
	}

	base := name
	if !strings.Contains(base, ":") && !strings.Contains(base, "@") {
		base = base + ":latest"
	}
	return "docker.io/library/" + base
}
