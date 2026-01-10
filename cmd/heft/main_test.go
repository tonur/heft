package main

import "testing"

// TestMainSymbolExists ensures the main symbol is present so the
// cmd/heft package has at least one test and participates normally in
// `go test ./...` runs.
func TestMainSymbolExists(t *testing.T) {
	_ = main
}
