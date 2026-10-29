package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// project lays out a root with DOCS.md listing docs and returns root and output.
func project(t *testing.T) (root, source, output string) {
	t.Helper()

	root = t.TempDir()
	source = filepath.Join(root, "docs")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "DOCS.md"), []byte("## Folders\n- docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return root, source, filepath.Join(root, "dist")
}

func TestRunGeneratesSite(t *testing.T) {
	root, source, output := project(t)
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

	count, err := Run(root, output)
	if err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Fatalf("got %d pages, want 2", count)
	}

	for path, expected := range map[string][]string{
		"docs/index.html": {
			"<h1>Welcome</h1>",
			`href="guides/start.html"`,
			"color-scheme",
		},
		"docs/guides/start.html": {
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

func TestRunRemovesStaleOutputFiles(t *testing.T) {
	root, source, output := project(t)
	if err := os.MkdirAll(filepath.Join(source, "images"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(source, "index.md"), []byte("# Welcome"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(source, "images", "logo.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(output, "old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(output, "old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(output, "images"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, stale := range []string{"old-page.html", filepath.Join("old", "x.txt"), filepath.Join("images", "ghost.png")} {
		if err := os.WriteFile(filepath.Join(output, stale), []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Run(root, output); err != nil {
		t.Fatal(err)
	}

	for _, stale := range []string{"old-page.html", "old", filepath.Join("images", "ghost.png")} {
		if _, err := os.Stat(filepath.Join(output, stale)); !os.IsNotExist(err) {
			t.Errorf("stale file %s must be removed: %v", stale, err)
		}
	}

	if _, err := os.Stat(filepath.Join(output, "docs", "index.html")); err != nil {
		t.Errorf("generated page missing: %v", err)
	}

	if _, err := os.Stat(filepath.Join(output, "docs", "images", "logo.png")); err != nil {
		t.Errorf("copied asset missing: %v", err)
	}
}

func TestRunCopiesAssetsAndIgnoresSymlinks(t *testing.T) {
	root, source, output := project(t)
	if err := os.MkdirAll(filepath.Join(source, "images"), 0o755); err != nil {
		t.Fatal(err)
	}

	for path, text := range map[string]string{
		"index.md":         "# Welcome",
		"styles.css":       "body { margin: 0 }",
		"images/logo.png":  "PNG bytes",
		"images/half.jpeg": "JPEG bytes",
	} {
		if err := os.WriteFile(filepath.Join(source, path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Symlink(filepath.Join(root, "styles.css"), filepath.Join(source, "shortcut.css")); err != nil {
		t.Fatal(err)
	}

	count, err := Run(root, output)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("got %d pages, want 1", count)
	}

	for _, path := range []string{"docs/styles.css", "docs/images/logo.png", "docs/images/half.jpeg"} {
		want, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(filepath.Join(output, path))
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != string(want) {
			t.Errorf("asset %s has content %q, want %q", path, got, want)
		}
	}

	if _, err := os.Stat(filepath.Join(output, "docs", "shortcut.css")); !os.IsNotExist(err) {
		t.Errorf("symlinked asset must not be copied: %v", err)
	}

	page, err := os.ReadFile(filepath.Join(output, "docs", "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(page), "Welcome") {
		t.Errorf("generated page missing: %s", page)
	}
}

func TestRunRejectsInvalidDirectories(t *testing.T) {
	root, source, _ := project(t)

	for _, output := range []string{source, filepath.Join(source, "dist"), root} {
		if _, err := Run(root, output); err == nil || !strings.Contains(err.Error(), "overlap") {
			t.Errorf("output %q: expected overlap error, got %v", output, err)
		}
	}

	if _, err := Run(root, filepath.Join(root, "dist")); err == nil {
		t.Error("folders without Markdown accepted")
	}

	if _, err := Run(filepath.Join(root, "missing"), filepath.Join(root, "dist")); err == nil {
		t.Error("missing root accepted")
	}
}

func TestRunRejectsSymlinkOverlap(t *testing.T) {
	root, source, _ := project(t)

	alias := filepath.Join(root, "alias")
	if err := os.Symlink(source, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := Run(root, filepath.Join(alias, "dist")); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected overlap error, got %v", err)
	}
}

func TestRunRejectsUnavailableModes(t *testing.T) {
	for _, mode := range []string{"module", "single"} {
		root, source, output := project(t)
		if err := os.WriteFile(filepath.Join(root, "DOCS.md"), []byte("## Folders\n- docs\n\n## Assemble\n"+mode+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(source, "index.md"), []byte("# Hi"), 0o644); err != nil {
			t.Fatal(err)
		}

		if _, err := Run(root, output); err == nil || !strings.Contains(err.Error(), "not available yet") {
			t.Errorf("mode %s: got %v", mode, err)
		}
	}
}

func TestRunWithoutConfigReadsOnlyRootFiles(t *testing.T) {
	root := t.TempDir()
	for path, text := range map[string]string{"README.md": "# Root", "docs/deep.md": "# Deep"} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	count, err := Run(root, filepath.Join(root, "dist"))
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("got %d pages, want 1", count)
	}

	if _, err := os.Stat(filepath.Join(root, "dist", "README.html")); err != nil {
		t.Errorf("root page missing: %v", err)
	}
}
