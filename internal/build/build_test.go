package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGeneratesSite(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "docs")
	output := filepath.Join(root, "dist")
	if err := os.MkdirAll(filepath.Join(source, "guides"), 0o755); err != nil {
		t.Fatal(err)
	}

	for path, text := range map[string]string{
		"index.md":        "# Welcome\n\n<script>alert('unsafe')</script>",
		"guides/start.md": "# Getting started\n\n**Hello**",
		"ignored.txt":     "ignored",
	} {
		if err := os.WriteFile(filepath.Join(source, path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	count, err := Run(source, output)
	if err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Fatalf("got %d pages, want 2", count)
	}

	for path, expected := range map[string][]string{
		"index.html": {
			"<h1>Welcome</h1>",
			`href="guides/start.html"`,
			"color-scheme",
		},
		"guides/start.html": {
			"<strong>Hello</strong>",
			`href="../index.html"`,
		},
	} {
		content, err := os.ReadFile(filepath.Join(output, path))
		if err != nil {
			t.Fatal(err)
		}

		for _, fragment := range expected {
			if !strings.Contains(string(content), fragment) {
				t.Errorf("%s missing %q", path, fragment)
			}
		}

		if strings.Contains(string(content), "<script>") {
			t.Errorf("%s contains raw script", path)
		}
	}
}

func TestRunRejectsInvalidDirectories(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "docs")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, output := range []string{source, filepath.Join(source, "dist"), root} {
		if _, err := Run(source, output); err == nil || !strings.Contains(err.Error(), "overlap") {
			t.Errorf("output %q: expected overlap error, got %v", output, err)
		}
	}

	if _, err := Run(source, filepath.Join(root, "dist")); err == nil {
		t.Error("empty source accepted")
	}

	if _, err := Run(filepath.Join(root, "missing"), filepath.Join(root, "dist")); err == nil {
		t.Error("missing source accepted")
	}
}

func TestRunRejectsSymlinkOverlap(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "docs")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}

	alias := filepath.Join(root, "alias")
	if err := os.Symlink(source, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := Run(source, filepath.Join(alias, "dist")); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected overlap error, got %v", err)
	}
}
