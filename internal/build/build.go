package build

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ArchL-Systems/docforge/internal/render"
)

// Run generates a site from Markdown. Source and output must be disjoint.
func Run(source, output string) (int, error) {
	if source == "" || output == "" {
		return 0, fmt.Errorf("source and output must not be empty")
	}

	source, err := resolvePath(source)
	if err != nil {
		return 0, err
	}

	info, err := os.Stat(source)
	if err != nil {
		return 0, fmt.Errorf("read source: %w", err)
	}

	if !info.IsDir() {
		return 0, fmt.Errorf("source must be a directory")
	}

	output, err = resolvePath(output)
	if err != nil {
		return 0, err
	}

	if contains(source, output) || contains(output, source) {
		return 0, fmt.Errorf("source and output directories must not overlap")
	}

	pages, err := readPages(source)
	if err != nil {
		return 0, fmt.Errorf("scan source: %w", err)
	}

	if len(pages) == 0 {
		return 0, fmt.Errorf("no Markdown files found in %s", source)
	}

	// The output directory is disjoint from the source (checked above), so
	// DocForge owns it: stale files from previous builds go away.
	if err := os.RemoveAll(output); err != nil {
		return 0, fmt.Errorf("clean output: %w", err)
	}

	// Assets go first so pages rendered afterwards win any name collision.
	if err := copyAssets(source, output); err != nil {
		return 0, fmt.Errorf("copy assets: %w", err)
	}

	if err := render.Site(output, pages); err != nil {
		return 0, fmt.Errorf("render site: %w", err)
	}

	return len(pages), nil
}

// copyAssets copies non-Markdown regular files so pages can reference
// images, styles and other resources by their source-relative path.
// Symlinks are ignored, mirroring the page scanner.
func copyAssets(source, output string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || filepath.Ext(path) == ".md" {
			return nil
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		destination := filepath.Join(output, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(destination, data, info.Mode().Perm())
	})
}

func readPages(source string) ([]render.Page, error) {
	var pages []render.Page

	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(path) != ".md" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		name := strings.TrimSuffix(relative, ".md")
		pages = append(pages, render.Page{
			Title:    filepath.ToSlash(name),
			Path:     name + ".html",
			Markdown: content,
		})

		return nil
	})

	return pages, err
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
