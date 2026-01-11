package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureHeftBinaryUsesEnv(t *testing.T) {
	tdir := t.TempDir()
	fake := filepath.Join(tdir, "heft-env")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho env-heft\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("HEFT_BINARY", fake)

	got, err := ensureHeftBinary(tdir)
	if err != nil {
		t.Fatalf("ensureHeftBinary error: %v", err)
	}
	if got != fake {
		t.Fatalf("ensureHeftBinary = %q, want %q", got, fake)
	}
}

func TestEnsureHeftBinaryUsesPath(t *testing.T) {
	tdir := t.TempDir()
	fake := filepath.Join(tdir, "heft")
	script := "#!/bin/sh\necho path-heft\n"
	if runtime.GOOS == "windows" {
		// On Windows tests, build a small Go binary instead.
		src := filepath.Join(tdir, "main.go")
		if err := os.WriteFile(src, []byte("package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"path-heft\")}"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		command := exec.Command("go", "build", "-o", fake, src)
		command.Env = os.Environ()
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build fake heft: %v\n%s", err, string(out))
		}
	} else {
		if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	t.Setenv("HEFT_BINARY", "")
	t.Setenv("PATH", tdir)

	got, err := ensureHeftBinary(tdir)
	if err != nil {
		t.Fatalf("ensureHeftBinary error: %v", err)
	}
	if got != fake {
		t.Fatalf("ensureHeftBinary = %q, want %q", got, fake)
	}
}
