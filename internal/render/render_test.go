package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderSingle(t *testing.T, markdown string) string {
	t.Helper()

	output := t.TempDir()
	pages := []Page{
		{Title: "Index", Path: "index.html", Markdown: []byte("# Index\n")},
		{
			Title:    "Start",
			Path:     "guides/start.html",
			Markdown: []byte(markdown),
		},
	}

	if err := Site(output, pages); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(output, "guides", "start.html"))
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func TestSiteRewritesMarkdownLinksToKnownPages(t *testing.T) {
	content := renderSingle(t, strings.Join([]string{
		"[Home](../index.md)",
		"[Same](start.md)",
		"[Fragment](../index.md#intro)",
	}, "\n"))

	for _, want := range []string{
		`href="../index.html"`,
		`href="start.html"`,
		`href="../index.html#intro"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q:\n%s", want, content)
		}
	}
}

func TestSitePreservesLinksOutsideGeneratedPages(t *testing.T) {
	content := renderSingle(t, strings.Join([]string{
		"[HTTP](https://example.com/README.md)",
		"[Protocol relative](//example.com/x.md)",
		"[Root](/abs.md)",
		"[Missing target](missing.md)",
		"[Similar extension](notes.markdown)",
	}, "\n"))

	for _, fragment := range []string{
		`href="https://example.com/README.md"`,
		`href="//example.com/x.md"`,
		`href="/abs.md"`,
		`href="missing.md"`,
		`href="notes.markdown"`,
	} {
		if !strings.Contains(content, fragment) {
			t.Errorf("content must keep %q:\n%s", fragment, content)
		}
	}
}

func TestSiteLeavesImagesUnrewritten(t *testing.T) {
	content := renderSingle(t, "![Diagram](diagram.md)")

	if strings.Contains(content, `href="diagram.html"`) {
		t.Errorf("image must not be rewritten as link:\n%s", content)
	}

	if !strings.Contains(content, `src="diagram.md"`) {
		t.Errorf("image src must stay untouched:\n%s", content)
	}
}

func TestNavigationLinkTitlesAreEscaped(t *testing.T) {
	output := t.TempDir()
	pages := []Page{
		{Title: `a&b "x"`, Path: "a.html", Markdown: []byte("# a\n")},
		{Title: "b", Path: "b.html", Markdown: []byte("# b\n")},
	}

	if err := Site(output, pages); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(output, "a.html"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), `>a&amp;b &#34;x&#34;</a>`) {
		t.Errorf("nav title not escaped properly:\n%s", content)
	}
}
