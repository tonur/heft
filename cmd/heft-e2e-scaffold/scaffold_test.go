package main

import "testing"

func TestSelectExpectedImagesPrefersHighConfidence(t *testing.T) {
	images := []scanImage{
		{Name: "high", Confidence: "high", Source: "rendered"},
		{Name: "medium", Confidence: "medium", Source: "static"},
	}

	selected := selectExpectedImages(images)
	if len(selected) != 1 {
		t.Fatalf("expected 1 image, got %d", len(selected))
	}
	if selected[0].Image == "" || selected[0].Confidence != "high" {
		t.Fatalf("unexpected selected image: %+v", selected[0])
	}
}

func TestSelectExpectedImagesFallsBackToAllWhenNoHigh(t *testing.T) {
	images := []scanImage{
		{Name: "medium", Confidence: "medium", Source: "static"},
		{Name: "low", Confidence: "low", Source: "regex"},
	}

	selected := selectExpectedImages(images)
	if len(selected) != 2 {
		t.Fatalf("expected 2 images, got %d", len(selected))
	}
}
