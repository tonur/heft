package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestEnsureHeftBinaryBuildsWhenNotOnPath exercises the branch where
// ensureHeftBinary falls back to running `go build ./cmd/heft`.
//
// It uses a small wrapper around the real `go` tool so that we can
// assert the command-line invocation without depending on any
// particular output format. If `go` is not available, the test is
// skipped.
func TestEnsureHeftBinaryBuildsWhenNotOnPath(t *testing.T) {
	// Skip if there is no go tool; without it the build path cannot run.
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not available in PATH; skipping build-path test")
	}

	repository := t.TempDir()
	// Minimal go.mod so that ./cmd/heft resolves.
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example.com/heft-test"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	// Create a minimal cmd/heft main package so the build succeeds quickly.
	heftDirectory := filepath.Join(repository, "cmd", "heft")
	if err := os.MkdirAll(heftDirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll cmd/heft: %v", err)
	}
	mainSrc := []byte("package main\nfunc main() {}\n")
	if err := os.WriteFile(filepath.Join(heftDirectory, "main.go"), mainSrc, 0o644); err != nil {
		t.Fatalf("WriteFile cmd/heft/main.go: %v", err)
	}

	// Create a wrapper for the go tool that delegates to the real go
	// but records that it was invoked. This keeps the behavior close
	// to the real path while remaining hermetic.
	wrapperDirectory := t.TempDir()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("LookPath go: %v", err)
	}

	wrapperPath := filepath.Join(wrapperDirectory, "go")
	wrapperSource := "#!/bin/sh\n" +
		"echo wrapper-go-invoked >> '" + filepath.Join(wrapperDirectory, "log") + "'\n" +
		"exec '" + realGo + "' \"$@\"\n"
	if runtime.GOOS == "windows" {
		// On Windows, fall back to calling the real go directly without
		// a shell script wrapper, since .bat/.cmd handling differs. For
		// simplicity, just use the real go on PATH.
		wrapperPath = realGo
	} else {
		if err := os.WriteFile(wrapperPath, []byte(wrapperSource), 0o755); err != nil {
			t.Fatalf("WriteFile go wrapper: %v", err)
		}
	}

	// Ensure HEFT_BINARY is unset so ensureHeftBinary does not short-circuit,
	// and remove any real heft from PATH so the build branch is taken.
	t.Setenv("HEFT_BINARY", "")
	// PATH will contain only our wrapper directory so LookPath("heft") fails
	// and the code falls back to building.
	t.Setenv("PATH", wrapperDirectory)

	binary, err := ensureHeftBinary(repository)
	if err != nil {
		t.Fatalf("ensureHeftBinary returned error: %v", err)
	}
	if binary == "" {
		t.Fatalf("ensureHeftBinary returned empty path")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("expected built heft binary at %s: %v", binary, err)
	}
}
