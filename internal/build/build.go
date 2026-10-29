package build

import (
	"fmt"
	"io/fs"
	"os"
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

	if cfg.Mode != config.ModePage {
		return 0, fmt.Errorf("assemble mode %q is not available yet", cfg.Mode)
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
	if err := copyAssets(root, cfg, output); err != nil {
		return 0, fmt.Errorf("copy assets: %w", err)
	}

	if err := render.Site(output, pages); err != nil {
		return 0, fmt.Errorf("render site: %w", err)
	}

	return len(pages), nil
}

// copyAssets copies non-Markdown regular files from the configured folders so
// pages can reference images, styles and other resources by their
// root-relative path. Symlinks are ignored, mirroring the collector.
func copyAssets(root string, cfg config.Config, output string) error {
	for _, folder := range cfg.Folders {
		base := filepath.Join(root, filepath.FromSlash(folder))
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() && cfg.Flat && path != base {
				return filepath.SkipDir
			}

			if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || filepath.Ext(path) == ".md" {
				return nil
			}

			relative, err := filepath.Rel(root, path)
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
		if err != nil {
			return err
		}
	}

	return nil
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
