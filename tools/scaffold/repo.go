package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// repositoryRootFunction is a variable to allow tests to stub repository
// discovery logic.
var repositoryRootFunction = repositoryRoot

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

// ensureHeftBinaryFunction is a variable to allow tests to stub heft binary
// resolution.
var ensureHeftBinaryFunction = ensureHeftBinary

// ensureHeftBinary builds a fresh binary with `go build ./cmd/heft` if none exists.
func ensureHeftBinary(repoRoot string) (string, error) {
	if p := os.Getenv("HEFT_BINARY"); p != "" {
		return p, nil
	}

	temporaryDirectory, err := os.MkdirTemp("", "heft-bin-")
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(temporaryDirectory, "heft")

	command := exec.Command("go", "build", "-o", binPath, "./cmd/heft")
	command.Dir = repoRoot
	command.Env = os.Environ()
	out, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("building heft failed: %v\n%s", err, string(out))
	}

	return binPath, nil
}
