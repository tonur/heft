package scan

import (
	"sort"
	"strings"
)

func splitRepoAndTag(name string) (repository string, hasTag bool) {
	// Very simple Docker-ish split: prefer digest (@), else last colon as tag
	// separator, but only if there is at least one '/' before it.
	if index := strings.Index(name, "@"); index != -1 {
		return name[:index], true
	}
	lastColon := strings.LastIndex(name, ":")
	slash := strings.Index(name, "/")
	if lastColon != -1 && slash != -1 && lastColon > slash {
		return name[:lastColon], true
	}
	return name, false
}

func dedupeImages(images []ImageFinding) []ImageFinding {
	seen := make(map[string]ImageFinding)
	for _, image := range images {
		repo, hasTag := splitRepoAndTag(image.Name)

		if existing, ok := seen[repo]; ok {
			// Prefer higher confidence.
			if existing.Confidence != image.Confidence {
				if existing.Confidence == ConfidenceHigh || (existing.Confidence == ConfidenceMedium && image.Confidence == ConfidenceLow) {
					continue
				}
			} else {
				// Same confidence: prefer tagged/digest over untagged.
				_, existingHasTag := splitRepoAndTag(existing.Name)
				if existingHasTag || !hasTag {
					continue
				}
			}
		}

		seen[repo] = image
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]ImageFinding, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}
