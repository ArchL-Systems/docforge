package render

import (
	"bytes"
	"embed"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

//go:embed templates/page.html assets/style.css
var assets embed.FS

var pageTemplate = template.Must(template.ParseFS(assets, "templates/page.html"))

type Page struct {
	Title    string
	Path     string
	Markdown []byte
}

type link struct {
	Title string
	URL   string
}

type pageData struct {
	Title   string
	Content template.HTML
	Style   template.CSS
	Links   []link
}

func Site(output string, pages []Page) error {
	style, err := assets.ReadFile("assets/style.css")
	if err != nil {
		return err
	}

	for _, page := range pages {
		var content bytes.Buffer
		markdown := goldmark.New()
		markdown.Parser().AddOptions(parser.WithASTTransformers(
			util.Prioritized(&mdLinkRewriter{current: page.Path, pages: pagePaths(pages)}, 1),
		))
		if err := markdown.Convert(page.Markdown, &content); err != nil {
			return err
		}

		links, err := navigationLinks(page.Path, pages)
		if err != nil {
			return err
		}

		// Goldmark's default renderer omits raw HTML and unsafe URLs.
		data := pageData{
			Title:   page.Title,
			Content: template.HTML(content.String()),
			Style:   template.CSS(style),
			Links:   links,
		}

		var document bytes.Buffer
		if err := pageTemplate.Execute(&document, data); err != nil {
			return err
		}

		destination := filepath.Join(output, page.Path)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(destination, document.Bytes(), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func navigationLinks(currentPath string, pages []Page) ([]link, error) {
	links := make([]link, 0, len(pages))

	for _, page := range pages {
		u, err := pageURL(currentPath, page.Path)
		if err != nil {
			return nil, err
		}

		links = append(links, link{
			Title: page.Title,
			URL:   u,
		})
	}

	return links, nil
}

func pageURL(current, target string) (string, error) {
	relative, err := filepath.Rel(filepath.Dir(current), target)
	if err != nil {
		return "", err
	}

	pageURL := url.URL{Path: filepath.ToSlash(relative)}
	return pageURL.String(), nil
}

// mdLinkRewriter rewrites relative Markdown links pointing at known pages so
// generated <a href> attributes keep visitors inside the generated site.
// Absolute URLs, fragments alone, unknown targets and images stay untouched.
type mdLinkRewriter struct {
	current string
	pages   map[string]bool
}

func (r *mdLinkRewriter) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if l, ok := n.(*ast.Link); ok {
			l.Destination = r.rewritten(l.Destination)
		}

		return ast.WalkContinue, nil
	})
}

func (r *mdLinkRewriter) rewritten(destination []byte) []byte {
	raw := string(destination)
	if raw == "" || absoluteURL(raw) {
		return destination
	}

	target, fragment, hasFragment := strings.Cut(raw, "#")
	if !strings.HasSuffix(target, ".md") {
		return destination
	}

	target = filepath.Join(filepath.Dir(r.current), filepath.FromSlash(target))
	target = strings.TrimSuffix(target, ".md") + ".html"
	if !r.pages[filepath.ToSlash(target)] {
		return destination
	}

	u, err := pageURL(r.current, target)
	if err != nil {
		return destination
	}

	if hasFragment {
		u += "#" + fragment
	}

	return []byte(u)
}

// absoluteURL reports destinations that must never be rewritten: full URLs,
// protocol-relative ones, in-page fragments and paths rooted at the site root.
func absoluteURL(destination string) bool {
	return strings.HasPrefix(destination, "http://") ||
		strings.HasPrefix(destination, "https://") ||
		strings.HasPrefix(destination, "mailto:") ||
		strings.HasPrefix(destination, "//") ||
		strings.HasPrefix(destination, "#") ||
		strings.HasPrefix(destination, "/") ||
		strings.Contains(destination, "://")
}

func pagePaths(pages []Page) map[string]bool {
	paths := make(map[string]bool, len(pages))
	for _, page := range pages {
		paths[filepath.ToSlash(page.Path)] = true
	}

	return paths
}
