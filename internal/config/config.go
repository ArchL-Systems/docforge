// Package config reads the DOCS.md file at the project root.
package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// File is the name of the configuration file inside the project root.
const File = "DOCS.md"

// Assemble modes accepted under "## Assemble".
const (
	ModePage   = "page"
	ModeModule = "module"
	ModeSingle = "single"
)

// Config describes which folders feed the documentation and how it is assembled.
type Config struct {
	// Folders are slash-separated paths relative to the root, in written order.
	Folders []string
	Mode    string
	// Flat is set when there is no DOCS.md: only the root itself is read,
	// without descending into subfolders.
	Flat bool
}

// Load reads DOCS.md from root. A missing file yields the default config.
func Load(root string) (Config, error) {
	content, err := os.ReadFile(filepath.Join(root, File))
	if errors.Is(err, fs.ErrNotExist) {
		return Config{Folders: []string{"."}, Mode: ModePage, Flat: true}, nil
	}

	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", File, err)
	}

	cfg, err := parse(content)
	if err != nil {
		return Config{}, err
	}

	for _, folder := range cfg.Folders {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(folder)))
		if err != nil || !info.IsDir() {
			return Config{}, fmt.Errorf("folder %q does not exist", folder)
		}
	}

	return cfg, nil
}

func parse(content []byte) (Config, error) {
	cfg := Config{Mode: ModePage}
	section := ""
	modeSeen := false

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if heading, ok := strings.CutPrefix(line, "## "); ok {
			section = strings.ToLower(strings.TrimSpace(heading))
			if section == "assemble" {
				cfg.Mode = ""
			}

			continue
		}

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch section {
		case "folders":
			item, ok := strings.CutPrefix(line, "-")
			if !ok {
				continue
			}

			folder, err := cleanFolder(item)
			if err != nil {
				return Config{}, err
			}

			cfg.Folders = append(cfg.Folders, folder)
		case "assemble":
			if modeSeen {
				continue
			}

			modeSeen = true
			cfg.Mode = line
		}
	}

	if err := scanner.Err(); err != nil {
		return Config{}, err
	}

	switch cfg.Mode {
	case ModePage, ModeModule, ModeSingle:
	default:
		return Config{}, fmt.Errorf("unknown assemble mode %q; use page, module or single", cfg.Mode)
	}

	if len(cfg.Folders) == 0 {
		cfg.Folders = []string{"."}
		cfg.Flat = true
	}

	return cfg, nil
}

// cleanFolder normalizes a folder entry and rejects empty or escaping paths.
func cleanFolder(item string) (string, error) {
	raw := strings.TrimSpace(item)
	if raw == "" {
		return "", fmt.Errorf("empty folder path in %s", File)
	}

	folder := path.Clean(filepath.ToSlash(raw))
	if path.IsAbs(folder) || folder == ".." || strings.HasPrefix(folder, "../") {
		return "", fmt.Errorf("folder %q must be inside the project root", raw)
	}

	return folder, nil
}
