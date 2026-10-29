package render

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ArchL-Systems/docforge/internal/config"
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
	// Source is the slash-separated path of the Markdown file relative to the
	// project root, and Module the configured folder it came from. Both are
	// only needed by the module and single assemble modes.
	Source string
	Module string
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

// Assemble writes the site using the given assemble mode: one HTML per file
// (page), one chapter per configured folder (module) or one document (single).
func Assemble(output string, pages []Page, mode string) error {
	switch mode {
	case "", config.ModePage:
		return Site(output, pages)
	case config.ModeModule, config.ModeSingle:
		return assembleDocuments(output, pages, mode)
	default:
		return fmt.Errorf("unknown assemble mode %q", mode)
	}
}

// Site writes one HTML page per Markdown file.
func Site(output string, pages []Page) error {
	targets := make(map[string]target, len(pages))
	for _, page := range pages {
		key := strings.TrimSuffix(filepath.ToSlash(page.Path), ".html") + ".md"
		targets[key] = target{doc: filepath.ToSlash(page.Path)}
	}

	for _, page := range pages {
		// Goldmark's default renderer omits raw HTML and unsafe URLs.
		content, err := convert(page.Markdown, &mdLinkRewriter{
			current: page.Path,
			dir:     path.Dir(filepath.ToSlash(page.Path)),
			targets: targets,
		}, nil)
		if err != nil {
			return err
		}

		links, err := navigationLinks(page.Path, pages)
		if err != nil {
			return err
		}

		if err := writeDocument(output, page.Path, page.Title, content, links); err != nil {
			return err
		}
	}

	return nil
}

// document is one generated HTML file made of one or more included files.
type document struct {
	Page
	sources []Page
}

func assembleDocuments(output string, pages []Page, mode string) error {
	var documents []*document
	byPath := make(map[string]*document)

	for _, page := range pages {
		docPath, title := "index.html", "Documentation"
		if mode == config.ModeModule {
			docPath, title = moduleDocument(page.Module)
		}

		doc, ok := byPath[docPath]
		if !ok {
			doc = &document{Page: Page{Title: title, Path: docPath}}
			byPath[docPath] = doc
			documents = append(documents, doc)
		}

		doc.sources = append(doc.sources, page)
	}

	targets := make(map[string]target, len(pages))
	sectionIDs := make(map[string]bool, len(pages))
	for _, doc := range documents {
		for _, page := range doc.sources {
			id := uniqueID(sectionID(page.Source), sectionIDs)
			targets[page.Source] = target{doc: doc.Path, anchor: id}
		}
	}

	navigation := make([]Page, 0, len(documents))
	for _, doc := range documents {
		navigation = append(navigation, doc.Page)
	}

	for _, doc := range documents {
		var content bytes.Buffer
		for _, page := range doc.sources {
			anchor := targets[page.Source].anchor
			html, err := convert(page.Markdown, &mdLinkRewriter{
				current:  doc.Path,
				dir:      path.Dir(page.Source),
				targets:  targets,
				combined: true,
			}, &prefixedIDs{prefix: anchor, used: make(map[string]int)})
			if err != nil {
				return err
			}

			fmt.Fprintf(&content, "<section id=%q>\n%s</section>\n", anchor, html)
		}

		links, err := navigationLinks(doc.Path, navigation)
		if err != nil {
			return err
		}

		if err := writeDocument(output, doc.Path, doc.Title, content.String(), links); err != nil {
			return err
		}
	}

	return nil
}

// moduleDocument names the chapter file for a configured folder.
func moduleDocument(module string) (docPath, title string) {
	if module == "" || module == "." {
		return "root.html", "."
	}

	return module + ".html", module
}

func convert(markdown []byte, rewriter *mdLinkRewriter, ids parser.IDs) (string, error) {
	options := []parser.Option{parser.WithASTTransformers(util.Prioritized(rewriter, 1))}
	var contextOptions []parser.ContextOption
	if ids != nil {
		options = append(options, parser.WithAutoHeadingID())
		contextOptions = append(contextOptions, parser.WithIDs(ids))
	}

	md := goldmark.New()
	md.Parser().AddOptions(options...)

	var content bytes.Buffer
	if err := md.Convert(markdown, &content, parser.WithContext(parser.NewContext(contextOptions...))); err != nil {
		return "", err
	}

	return content.String(), nil
}

func writeDocument(output, docPath, title, content string, links []link) error {
	style, err := assets.ReadFile("assets/style.css")
	if err != nil {
		return err
	}

	data := pageData{
		Title:   title,
		Content: template.HTML(content),
		Style:   template.CSS(style),
		Links:   links,
	}

	var document bytes.Buffer
	if err := pageTemplate.Execute(&document, data); err != nil {
		return err
	}

	destination := filepath.Join(output, filepath.FromSlash(docPath))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	return os.WriteFile(destination, document.Bytes(), 0o644)
}

// sectionID derives the anchor of an included file inside a joined document.
func sectionID(source string) string {
	return "src-" + slug(source)
}

func uniqueID(id string, used map[string]bool) string {
	candidate := id
	for n := 2; used[candidate]; n++ {
		candidate = fmt.Sprintf("%s-%d", id, n)
	}

	used[candidate] = true

	return candidate
}

// slug lowercases text and keeps letters and digits, turning separators into
// hyphens, the same shape as GitHub heading anchors.
func slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/' || r == '.':
			b.WriteByte('-')
		}
	}

	return strings.Trim(b.String(), "-")
}

// prefixedIDs generates heading ids unique across a joined document by
// prefixing them with the id of the section they belong to.
type prefixedIDs struct {
	prefix string
	used   map[string]int
}

func (p *prefixedIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	id := slug(string(value))
	if id == "" {
		id = "section"
	}

	n := p.used[id]
	p.used[id]++
	if n > 0 {
		id = fmt.Sprintf("%s-%d", id, n)
	}

	return []byte(p.prefix + "-" + id)
}

func (p *prefixedIDs) Put(value []byte) {
	p.used[string(value)]++
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

// Images returns the destination of every image cited in a Markdown file.
func Images(markdown []byte) []string {
	root := goldmark.New().Parser().Parse(text.NewReader(markdown))

	var images []string
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if image, ok := n.(*ast.Image); ok && entering {
			images = append(images, string(image.Destination))
		}

		return ast.WalkContinue, nil
	})

	return images
}

// target is where an included Markdown file ends up: its output document and,
// in joined documents, the id of its section.
type target struct {
	doc    string
	anchor string
}

// mdLinkRewriter rewrites relative Markdown links pointing at included files
// so generated <a href> attributes keep visitors inside the generated site.
// Absolute URLs, fragments alone, unknown targets and images stay untouched.
type mdLinkRewriter struct {
	// current is the output document being rendered and dir the slash-separated
	// directory links are resolved against (that of the source file).
	current string
	dir     string
	targets map[string]target
	// combined points links at section anchors of joined documents.
	combined bool
}

func (r *mdLinkRewriter) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
	// The walker function always returns nil, so the error is impossible;
	// the explicit assignment documents that and satisfies errcheck.
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Link:
			node.Destination = r.rewritten(node.Destination)
		case *ast.Image:
			node.Destination = r.rebased(node.Destination)
		}

		return ast.WalkContinue, nil
	})
}

// rebased points an image of a joined document at its root-relative location,
// since the document lives in a different directory than the cited file.
func (r *mdLinkRewriter) rebased(destination []byte) []byte {
	raw := string(destination)
	if !r.combined || raw == "" || IsAbsoluteURL(raw) {
		return destination
	}

	relative, err := filepath.Rel(path.Dir(filepath.ToSlash(r.current)), path.Join(r.dir, raw))
	if err != nil {
		return destination
	}

	return []byte(filepath.ToSlash(relative))
}

func (r *mdLinkRewriter) rewritten(destination []byte) []byte {
	raw := string(destination)
	if raw == "" || IsAbsoluteURL(raw) {
		return destination
	}

	file, fragment, hasFragment := strings.Cut(raw, "#")
	if !strings.HasSuffix(file, ".md") {
		return destination
	}

	found, ok := r.targets[path.Join(r.dir, file)]
	if !ok {
		return destination
	}

	u, err := pageURL(r.current, found.doc)
	if err != nil {
		return destination
	}

	if !r.combined {
		if hasFragment {
			u += "#" + fragment
		}

		return []byte(u)
	}

	if found.doc == filepath.ToSlash(r.current) {
		u = ""
	}

	anchor := found.anchor
	if fragment != "" {
		anchor += "-" + fragment
	}

	return []byte(u + "#" + anchor)
}

// IsAbsoluteURL reports destinations that must never be rewritten: full URLs,
// protocol-relative ones, in-page fragments and paths rooted at the site root.
func IsAbsoluteURL(destination string) bool {
	return strings.HasPrefix(destination, "http://") ||
		strings.HasPrefix(destination, "https://") ||
		strings.HasPrefix(destination, "mailto:") ||
		strings.HasPrefix(destination, "//") ||
		strings.HasPrefix(destination, "#") ||
		strings.HasPrefix(destination, "/") ||
		strings.Contains(destination, "://")
}
