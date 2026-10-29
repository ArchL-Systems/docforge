package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"build", "--help"}} {
		var stdout, stderr bytes.Buffer
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(stdout.String()+stderr.String(), "Usage") {
			t.Fatal("missing help")
		}
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"build", "--unknown"}, {"build", "extra"}, {"build", "--source", "docs"}} {
		var stdout, stderr bytes.Buffer
		if err := Run(args, &stdout, &stderr); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestBuildUsesWorkingDirectoryAsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	for path, text := range map[string]string{"DOCS.md": "## Folders\n- docs\n", "docs/index.md": "# Hi"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(root)

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"build", "--output", "out"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "out", "docs", "index.html")); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(stdout.String(), "Generated 1 pages") {
		t.Fatalf("output = %q", stdout.String())
	}
}
