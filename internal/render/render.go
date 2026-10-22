package render

import (
	"bytes"
	"embed"
	"html/template"
	"net/url"
	"os"
	"path/filepath"

	"github.com/yuin/goldmark"
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
		if err := goldmark.Convert(page.Markdown, &content); err != nil {
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
		relativePath, err := filepath.Rel(filepath.Dir(currentPath), page.Path)
		if err != nil {
			return nil, err
		}

		pageURL := url.URL{Path: filepath.ToSlash(relativePath)}
		links = append(links, link{
			Title: page.Title,
			URL:   pageURL.String(),
		})
	}

	return links, nil
}
