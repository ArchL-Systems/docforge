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

var indexTemplate = template.Must(template.New("index").Parse(`<h1>DocForge</h1>
{{range .}}<section>
{{if .Name}}<h2>{{if .URL}}<a href="{{.URL}}">{{.Name}}</a>{{else}}{{.Name}}{{end}}</h2>
{{end}}<ul>
{{range .Links}}<li><a href="{{.URL}}">{{.Title}}</a> <code>{{.Source}}</code></li>
{{end}}</ul>
</section>
{{end}}`))

type link struct {
	Title   string
	URL     string
	Source  string
	Current bool
}

// navGroup is one module in the navigation. URL is set when the module is a
// chapter of its own, and Current marks the group holding the open page.
type navGroup struct {
	Name    string
	URL     string
	Current bool
	Links   []link
}

type pageData struct {
	Title   string
	Content template.HTML
	Style   template.CSS
	Groups  []navGroup
	Home    string
	Source  string
}

// Title returns the text of the first heading of a Markdown file, or "" when
// it has none.
func Title(markdown []byte) string {
	root := goldmark.New().Parser().Parse(text.NewReader(markdown))

	var title string
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(*ast.Heading); ok && entering {
			title = strings.TrimSpace(plainText(n, markdown))
			return ast.WalkStop, nil
		}

		return ast.WalkContinue, nil
	})

	return title
}

func plainText(n ast.Node, source []byte) string {
	var b strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch c := child.(type) {
		case *ast.Text:
			b.Write(c.Segment.Value(source))
		case *ast.String:
			b.Write(c.Value)
		default:
			b.WriteString(plainText(child, source))
		}
	}

	return b.String()
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

		groups, err := pageGroups(page.Path, pages)
		if err != nil {
			return err
		}

		if err := writeDocument(output, page.Path, page.Title, page.Source, content, groups); err != nil {
			return err
		}
	}

	return writeIndex(output, hasPath(pages, indexPath), func() ([]navGroup, error) {
		return pageGroups(indexPath, pages)
	})
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
		docPath, title := indexPath, "Documentation"
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

			fmt.Fprintf(&content, "<section id=%q>\n<p class=\"source\">%s</p>\n%s</section>\n",
				anchor, template.HTMLEscapeString(page.Source), html)
		}

		groups, err := chapterGroups(doc.Path, documents, targets)
		if err != nil {
			return err
		}

		if err := writeDocument(output, doc.Path, doc.Title, "", content.String(), groups); err != nil {
			return err
		}
	}

	return writeIndex(output, byPath[indexPath] != nil, func() ([]navGroup, error) {
		return chapterGroups(indexPath, documents, targets)
	})
}

const indexPath = "index.html"

func hasPath(pages []Page, docPath string) bool {
	for _, page := range pages {
		if filepath.ToSlash(page.Path) == docPath {
			return true
		}
	}

	return false
}

// writeIndex generates index.html listing every module and page unless the
// documentation already provides one.
func writeIndex(output string, exists bool, groups func() ([]navGroup, error)) error {
	if exists {
		return nil
	}

	navigation, err := groups()
	if err != nil {
		return err
	}

	var content bytes.Buffer
	if err := indexTemplate.Execute(&content, navigation); err != nil {
		return err
	}

	return writeDocument(output, indexPath, "Início", "", content.String(), navigation)
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

func writeDocument(output, docPath, title, source, content string, groups []navGroup) error {
	style, err := assets.ReadFile("assets/style.css")
	if err != nil {
		return err
	}

	home, err := pageURL(docPath, indexPath)
	if err != nil {
		return err
	}

	data := pageData{
		Title:   title,
		Content: template.HTML(content),
		Style:   template.CSS(style),
		Groups:  groups,
		Home:    home,
		Source:  source,
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

// pageGroups lists every page grouped by module, in order, linking each one
// relative to the page being rendered.
func pageGroups(current string, pages []Page) ([]navGroup, error) {
	var groups []navGroup
	index := make(map[string]int)

	for _, page := range pages {
		u, err := pageURL(current, page.Path)
		if err != nil {
			return nil, err
		}

		i, ok := index[page.Module]
		if !ok {
			i = len(groups)
			index[page.Module] = i
			groups = append(groups, navGroup{Name: moduleName(page.Module)})
		}

		isCurrent := filepath.ToSlash(page.Path) == filepath.ToSlash(current)
		groups[i].Current = groups[i].Current || isCurrent
		groups[i].Links = append(groups[i].Links, link{Title: page.Title, URL: u, Source: page.Source, Current: isCurrent})
	}

	return groups, nil
}

// chapterGroups lists the joined documents, each linking to the sections of
// the files it holds.
func chapterGroups(current string, documents []*document, targets map[string]target) ([]navGroup, error) {
	groups := make([]navGroup, 0, len(documents))

	for _, doc := range documents {
		u, err := pageURL(current, doc.Path)
		if err != nil {
			return nil, err
		}

		group := navGroup{Name: moduleName(doc.Title), URL: u, Current: doc.Path == filepath.ToSlash(current)}
		for _, page := range doc.sources {
			href := u + "#" + targets[page.Source].anchor
			if group.Current {
				href = "#" + targets[page.Source].anchor
			}

			group.Links = append(group.Links, link{Title: page.Title, URL: href, Source: page.Source})
		}

		groups = append(groups, group)
	}

	return groups, nil
}

func moduleName(module string) string {
	if module == "." {
		return "Raiz"
	}

	return module
}

func pageURL(current, target string) (string, error) {
	relative, err := filepath.Rel(filepath.Dir(current), target)
	if err != nil {
		return "", err
	}

	pageURL := url.URL{Path: filepath.ToSlash(relative)}
	return pageURL.String(), nil
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
