package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// repositoryRootFunc is a variable to allow tests to stub repository
// discovery logic.
var repositoryRootFunc = repositoryRoot

// repositoryRoot walks up from the current working directory until it
// finds a go.mod file, treating that directory as the repository root.
func repositoryRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod from %s", cwd)
		}
		dir = parent
	}
}

// ensureHeftBinaryFunc is a variable to allow tests to stub heft binary
// resolution.
var ensureHeftBinaryFunc = ensureHeftBinary

// ensureHeftBinary locates or builds a heft binary to use for
// scaffolding. It prefers HEFT_BINARY, then an existing heft on PATH,
// and finally builds a fresh binary with `go build ./cmd/heft`.
func ensureHeftBinary(repoRoot string) (string, error) {
	if p := os.Getenv("HEFT_BINARY"); p != "" {
		return p, nil
	}

	if p, err := exec.LookPath("heft"); err == nil {
		return p, nil
	}

	tmpDir, err := os.MkdirTemp("", "heft-e2e-scaffold-")
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(tmpDir, "heft")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/heft")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("building heft failed: %v\n%s", err, string(out))
	}

	return binPath, nil
}
