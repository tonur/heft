package main

import "testing"

func TestChooseBestVersionPrefersHighestNumeric(t *testing.T) {
	entries := []versionedEntry{
		{major: 0, minor: 9, patch: 0, idx: 0},
		{major: 1, minor: 0, patch: 0, idx: 1},
		{major: 0, minor: 10, patch: 0, idx: 2},
	}

	best := chooseBestVersion(entries)
	if best.major != 1 || best.minor != 0 || best.patch != 0 {
		t.Fatalf("expected 1.0.0, got %d.%d.%d", best.major, best.minor, best.patch)
	}
	if best.idx != 1 {
		t.Fatalf("expected idx 1, got %d", best.idx)
	}
}

func TestChooseBestVersionHandlesPreReleasesViaCaller(t *testing.T) {
	// chooseBestVersion is unaware of pre-release status beyond fields
	// provided by parseVersionToEntry; this test ensures it still
	// compares numeric components correctly when isPre is set.
	entries := []versionedEntry{
		{major: 1, minor: 0, patch: 0, isPre: true, idx: 0},
		{major: 1, minor: 1, patch: 0, isPre: true, idx: 1},
	}

	best := chooseBestVersion(entries)
	if best.major != 1 || best.minor != 1 || best.patch != 0 || !best.isPre {
		t.Fatalf("expected 1.1.0 pre-release, got %+v", best)
	}
}
