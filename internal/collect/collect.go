// Package collect gathers the Markdown files named by the DOCS.md config.
package collect

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ArchL-Systems/docforge/internal/config"
)

// File is one Markdown file taken into the documentation.
type File struct {
	// Folder is the configured folder the file came from.
	Folder string
	// Source is the slash-separated path relative to the project root.
	Source string
}

// Files walks only the configured folders and returns their Markdown files
// in documentation order: folders as written, then by path inside each one.
// Symlinks are not followed. A file reached from two folders is an error.
func Files(root string, cfg config.Config) ([]File, error) {
	var files []File
	seen := make(map[string]string)

	for _, folder := range cfg.Folders {
		found, err := walkFolder(root, folder, cfg.Flat)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", folder, err)
		}

		for _, file := range found {
			if first, ok := seen[file.Source]; ok {
				return nil, fmt.Errorf("%s is included by both %q and %q", file.Source, first, folder)
			}

			seen[file.Source] = folder
			files = append(files, file)
		}
	}

	return files, nil
}

func walkFolder(root, folder string, flat bool) ([]File, error) {
	base := filepath.Join(root, filepath.FromSlash(folder))
	var files []File

	err := filepath.WalkDir(base, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if flat && current != base {
				return filepath.SkipDir
			}

			return nil
		}

		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !strings.HasSuffix(current, ".md") {
			return nil
		}

		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}

		files = append(files, File{Folder: folder, Source: path.Clean(filepath.ToSlash(relative))})

		return nil
	})

	return files, err
}
