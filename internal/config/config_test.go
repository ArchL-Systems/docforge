package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func project(t *testing.T, docs string, folders ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, folder := range folders {
		if err := os.MkdirAll(filepath.Join(root, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if docs != "" {
		if err := os.WriteFile(filepath.Join(root, File), []byte(docs), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func TestLoadReadsFoldersInOrderAndMode(t *testing.T) {
	root := project(t, "# DocForge\n\n## Folders\n- internal/orders\n- internal/billing\n\n## Assemble\nmodule\n",
		"internal/billing", "internal/orders")

	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"internal/orders", "internal/billing"}; !reflect.DeepEqual(cfg.Folders, want) {
		t.Fatalf("folders = %v, want %v", cfg.Folders, want)
	}

	if cfg.Mode != ModeModule || cfg.Flat {
		t.Fatalf("got mode %q flat %v", cfg.Mode, cfg.Flat)
	}
}

func TestLoadDefaultsModeToPage(t *testing.T) {
	root := project(t, "# DocForge\n\n## Folders\n- docs\n", "docs")

	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Mode != ModePage {
		t.Fatalf("mode = %q, want page", cfg.Mode)
	}
}

func TestLoadWithoutFileReadsRootOnly(t *testing.T) {
	cfg, err := Load(project(t, ""))
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"."}; !reflect.DeepEqual(cfg.Folders, want) || cfg.Mode != ModePage || !cfg.Flat {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoadErrorsCiteTheValue(t *testing.T) {
	for name, test := range map[string]struct {
		docs, want string
	}{
		"invalid mode":   {"## Folders\n- docs\n\n## Assemble\nbook\n", `"book"`},
		"empty mode":     {"## Folders\n- docs\n\n## Assemble\n", `""`},
		"empty path":     {"## Folders\n- \n", "empty folder"},
		"missing folder": {"## Folders\n- nope\n", `"nope"`},
		"outside root":   {"## Folders\n- ../other\n", `"../other"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(project(t, test.docs, "docs"))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %s", err, test.want)
			}
		})
	}
}
