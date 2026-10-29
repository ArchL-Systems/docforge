package collect

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ArchL-Systems/docforge/internal/config"
)

func write(t *testing.T, root string, paths ...string) {
	t.Helper()

	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte("# x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func sources(files []File) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Source)
	}

	return out
}

func TestFilesWalksSubfoldersAnyMarkdownName(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/design.md", "a/sub/deep.md", "a/notes.txt", "b/README.md", "other/skip.md")

	files, err := Files(root, config.Config{Folders: []string{"b", "a"}, Mode: config.ModePage})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"b/README.md", "a/design.md", "a/sub/deep.md"}
	if got := sources(files); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if files[0].Folder != "b" {
		t.Fatalf("folder = %q, want b", files[0].Folder)
	}
}

func TestFilesFlatRootDoesNotDescend(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "docs/guide.md")

	files, err := Files(root, config.Config{Folders: []string{"."}, Mode: config.ModePage, Flat: true})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := sources(files), []string{"README.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilesRejectsOverlappingFolders(t *testing.T) {
	root := t.TempDir()
	write(t, root, "internal/billing/api.md")

	_, err := Files(root, config.Config{Folders: []string{"internal", "internal/billing"}})
	if err == nil || !strings.Contains(err.Error(), "internal/billing/api.md") {
		t.Fatalf("error = %v, want it to name the repeated file", err)
	}
}

func TestFilesSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/real.md", "elsewhere/linked.md")

	if err := os.Symlink(filepath.Join(root, "elsewhere", "linked.md"), filepath.Join(root, "docs", "link.md")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "docs", "dir")); err != nil {
		t.Fatal(err)
	}

	files, err := Files(root, config.Config{Folders: []string{"docs"}})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := sources(files), []string{"docs/real.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
