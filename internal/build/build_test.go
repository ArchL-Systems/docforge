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

	if err := os.WriteFile(filepath.Join(source, "index.md"), []byte("# Welcome\n\n![logo](images/logo.png)"), 0o644); err != nil {
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

func TestRunCopiesOnlyCitedImages(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"DOCS.md":                "## Folders\n- docs\n",
		"docs/index.md":          "# Welcome\n\n![logo](images/logo.png) ![half](images/half.JPEG?raw=1)\n\n![web](https://example.com/x.png)",
		"docs/images/logo.png":   "PNG bytes",
		"docs/images/half.JPEG":  "JPEG bytes",
		"docs/images/unused.png": "unused",
		"docs/main.go":           "package docs",
		"docs/schema.sql":        "select 1;",
		"docs/styles.css":        "body { margin: 0 }",
	})

	output := filepath.Join(root, "dist")
	if _, err := Run(root, output); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"docs/images/logo.png", "docs/images/half.JPEG"} {
		if got, want := read(t, filepath.Join(output, path)), read(t, filepath.Join(root, path)); got != want {
			t.Errorf("image %s has content %q, want %q", path, got, want)
		}
	}

	for _, path := range []string{"docs/images/unused.png", "docs/main.go", "docs/schema.sql", "docs/styles.css"} {
		if _, err := os.Stat(filepath.Join(output, path)); !os.IsNotExist(err) {
			t.Errorf("%s must not be copied: %v", path, err)
		}
	}

	if page := read(t, filepath.Join(output, "docs", "index.html")); !strings.Contains(page, `src="images/logo.png"`) {
		t.Errorf("relative image path changed:\n%s", page)
	}
}

func TestRunFailsOnMissingCitedImage(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"DOCS.md":       "## Folders\n- docs\n",
		"docs/index.md": "![gone](images/gone.png)",
	})

	_, err := Run(root, filepath.Join(root, "dist"))
	if err == nil || !strings.Contains(err.Error(), "docs/images/gone.png") {
		t.Fatalf("error = %v, want it to cite the missing path", err)
	}
}

func TestRunKeepsImagesResolvingInJoinedDocuments(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"DOCS.md":              "## Folders\n- billing\n\n## Assemble\nmodule\n",
		"billing/api.md":       "# API\n\n![flow](img/flow.svg)",
		"billing/img/flow.svg": "<svg/>",
	})

	output := filepath.Join(root, "dist")
	if _, err := Run(root, output); err != nil {
		t.Fatal(err)
	}

	if page := read(t, filepath.Join(output, "billing.html")); !strings.Contains(page, `src="billing/img/flow.svg"`) {
		t.Errorf("image path not rebased:\n%s", page)
	}

	if _, err := os.Stat(filepath.Join(output, "billing", "img", "flow.svg")); err != nil {
		t.Errorf("image missing from output: %v", err)
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

func TestRunAssemblesModules(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"DOCS.md":          "## Folders\n- billing\n- orders\n\n## Assemble\nmodule\n",
		"billing/api.md":   "# Billing API\n\nSee [orders](../orders/flow.md#steps).",
		"billing/notes.md": "# Notes\n\nBack to [API](api.md) or [outside](../README.md).",
		"orders/flow.md":   "# Flow\n\n## Steps\n",
		"README.md":        "# Not included",
	})

	count, err := Run(root, filepath.Join(root, "dist"))
	if err != nil {
		t.Fatal(err)
	}

	if count != 3 {
		t.Fatalf("got %d pages, want 3", count)
	}

	billing := read(t, filepath.Join(root, "dist", "billing.html"))
	for _, want := range []string{
		`<section id="src-billing-api-md">`,
		`<section id="src-billing-notes-md">`,
		`href="orders.html#src-orders-flow-md-steps"`,
		`href="#src-billing-api-md"`,
		`href="../README.md"`,
	} {
		if !strings.Contains(billing, want) {
			t.Errorf("billing.html missing %q:\n%s", want, billing)
		}
	}

	orders := read(t, filepath.Join(root, "dist", "orders.html"))
	if !strings.Contains(orders, `id="src-orders-flow-md-steps"`) {
		t.Errorf("orders.html missing heading id:\n%s", orders)
	}
}

func TestRunAssemblesSingleDocument(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"DOCS.md":        "## Folders\n- billing\n- orders\n\n## Assemble\nsingle\n",
		"billing/api.md": "# API\n\n## Intro\n\nSee [flow](../orders/flow.md).",
		"orders/flow.md": "# Flow\n\n## Intro\n\n[API intro](../billing/api.md#intro)",
	})

	if _, err := Run(root, filepath.Join(root, "dist")); err != nil {
		t.Fatal(err)
	}

	index := read(t, filepath.Join(root, "dist", "index.html"))
	for _, want := range []string{
		`href="#src-orders-flow-md"`,
		`href="#src-billing-api-md-intro"`,
		`id="src-billing-api-md-intro"`,
		`id="src-orders-flow-md-intro"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q:\n%s", want, index)
		}
	}

	if strings.Index(index, "src-billing-api-md\"") > strings.Index(index, "src-orders-flow-md\"") {
		t.Error("folders out of order")
	}
}

func TestRunPageModeRewritesLinksBetweenIncludedFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"DOCS.md":        "## Folders\n- billing\n- orders\n",
		"billing/api.md": "# API\n\n[flow](../orders/flow.md#steps) [out](../README.md) [web](https://x.dev/a.md)",
		"orders/flow.md": "# Flow",
		"README.md":      "# Out",
	})

	if _, err := Run(root, filepath.Join(root, "dist")); err != nil {
		t.Fatal(err)
	}

	api := read(t, filepath.Join(root, "dist", "billing", "api.html"))
	for _, want := range []string{`href="../orders/flow.html#steps"`, `href="../README.md"`, `href="https://x.dev/a.md"`} {
		if !strings.Contains(api, want) {
			t.Errorf("api.html missing %q:\n%s", want, api)
		}
	}
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, text := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
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
