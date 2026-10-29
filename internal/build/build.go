package build

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ArchL-Systems/docforge/internal/collect"
	"github.com/ArchL-Systems/docforge/internal/config"
	"github.com/ArchL-Systems/docforge/internal/render"
)

// Run generates a site from the Markdown folders listed in root's DOCS.md.
// The output must not overlap any of those folders.
func Run(root, output string) (int, error) {
	if root == "" || output == "" {
		return 0, fmt.Errorf("root and output must not be empty")
	}

	root, err := resolvePath(root)
	if err != nil {
		return 0, err
	}

	info, err := os.Stat(root)
	if err != nil {
		return 0, fmt.Errorf("read root: %w", err)
	}

	if !info.IsDir() {
		return 0, fmt.Errorf("root must be a directory")
	}

	output, err = resolvePath(output)
	if err != nil {
		return 0, err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return 0, err
	}

	for _, folder := range cfg.Folders {
		folderPath := filepath.Join(root, filepath.FromSlash(folder))
		// A flat root reads only its direct files, so an output inside
		// it is safe; the output may still never contain the root.
		overlaps := contains(folderPath, output) || contains(output, folderPath)
		if cfg.Flat {
			overlaps = contains(output, folderPath)
		}

		if overlaps {
			return 0, fmt.Errorf("output directory must not overlap folder %q", folder)
		}
	}

	files, err := collect.Files(root, cfg)
	if err != nil {
		return 0, fmt.Errorf("scan root: %w", err)
	}

	if len(files) == 0 {
		return 0, fmt.Errorf("no Markdown files found in %s", root)
	}

	pages, err := readPages(root, files)
	if err != nil {
		return 0, fmt.Errorf("read pages: %w", err)
	}

	// The output directory is disjoint from the folders (checked above), so
	// DocForge owns it: stale files from previous builds go away.
	if err := os.RemoveAll(output); err != nil {
		return 0, fmt.Errorf("clean output: %w", err)
	}

	// Assets go first so pages rendered afterwards win any name collision.
	if err := copyAssets(root, pages, output); err != nil {
		return 0, fmt.Errorf("copy assets: %w", err)
	}

	if err := render.Assemble(output, pages, cfg.Mode); err != nil {
		return 0, fmt.Errorf("render site: %w", err)
	}

	return len(pages), nil
}

// imageExtensions are the only asset types copied to the output.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
}

// copyAssets copies the images cited by the included Markdown, keeping their
// root-relative paths. Cited images that are missing fail the build.
func copyAssets(root string, pages []render.Page, output string) error {
	copied := make(map[string]bool)

	for _, page := range pages {
		for _, reference := range render.Images(page.Markdown) {
			relative, ok := imagePath(page.Source, reference)
			if !ok || copied[relative] {
				continue
			}

			copied[relative] = true

			if err := copyImage(root, relative, output); err != nil {
				return fmt.Errorf("%s: image %q: %w", page.Source, reference, err)
			}
		}
	}

	return nil
}

// imagePath resolves an image reference against the Markdown file citing it.
// It reports false for URLs, root-absolute paths and unsupported extensions.
func imagePath(source, reference string) (string, bool) {
	if render.IsAbsoluteURL(reference) {
		return "", false
	}

	reference, _, _ = strings.Cut(reference, "#")
	reference, _, _ = strings.Cut(reference, "?")

	decoded, err := url.PathUnescape(reference)
	if err != nil {
		decoded = reference
	}

	if !imageExtensions[strings.ToLower(path.Ext(decoded))] {
		return "", false
	}

	return path.Join(path.Dir(source), decoded), true
}

func copyImage(root, relative, output string) error {
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return fmt.Errorf("%s is outside the project root", relative)
	}

	source := filepath.Join(root, filepath.FromSlash(relative))

	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("%s not found", relative)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", relative)
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	destination := filepath.Join(output, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	return os.WriteFile(destination, data, info.Mode().Perm())
}

func readPages(root string, files []collect.File) ([]render.Page, error) {
	pages := make([]render.Page, 0, len(files))

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Source)))
		if err != nil {
			return nil, err
		}

		name := strings.TrimSuffix(file.Source, ".md")
		pages = append(pages, render.Page{
			Title:    name,
			Path:     filepath.FromSlash(name) + ".html",
			Markdown: content,
			Source:   file.Source,
			Module:   file.Folder,
		})
	}

	return pages, nil
}

func contains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil || relative == ".." {
		return false
	}

	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// Resolve existing ancestors so symlinks cannot bypass the overlap check.
func resolvePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}

	if !os.IsNotExist(err) {
		return "", err
	}

	parent, err := resolvePath(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}

	return filepath.Join(parent, filepath.Base(absolute)), nil
}
