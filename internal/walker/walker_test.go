package walker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/it1ro/cake/pkg/types"
)

func TestTree(t *testing.T) {
	tests := []struct {
		name    string
		entries []types.FileEntry
		want    string
	}{
		{
			name:    "empty",
			entries: nil,
			want:    "",
		},
		{
			name:    "single_file",
			entries: []types.FileEntry{{Path: "main.go"}},
			want:    "└── main.go\n",
		},
		{
			name: "nested_tree",
			entries: []types.FileEntry{
				{Path: "README.md"},
				{Path: "cmd/cake/main.go"},
				{Path: "internal/walker/walker.go"},
				{Path: "internal/walker/walker_test.go"},
				{Path: "pkg/types/types.go"},
			},
			want: "├── README.md\n" +
				"├── cmd/\n" +
				"│   └── cake/\n" +
				"│       └── main.go\n" +
				"├── internal/\n" +
				"│   └── walker/\n" +
				"│       ├── walker.go\n" +
				"│       └── walker_test.go\n" +
				"└── pkg/\n" +
				"    └── types/\n" +
				"        └── types.go\n",
		},
		{
			// Соседние файлы и дирректории в одной папке:
			// проверяем корректность вертикальных префиксов.
			name: "siblings_mixed",
			entries: []types.FileEntry{
				{Path: "a/file.go"},
				{Path: "b.go"},
				{Path: "c/file.go"},
			},
			want: "├── a/\n" +
				"│   └── file.go\n" +
				"├── b.go\n" +
				"└── c/\n" +
				"    └── file.go\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Tree(tc.entries)
			if got != tc.want {
				t.Errorf("mismatch\n--- want ---\n%s\n--- got ---\n%s", tc.want, got)
			}
		})
	}
}

func TestWalk_RespectsGitignore(t *testing.T) {
	root := t.TempDir()

	// Файлы
	mustWrite(t, root, "keep.go", "package main\n")
	mustWrite(t, root, "skip.go", "package main\n")
	mustWrite(t, root, ".gitignore", "skip.go\n")

	entries, err := Walk(Options{Root: root, UseGitignore: true})
	if err != nil {
		t.Fatal(err)
	}

	var paths []string
	for _, e := range entries {
		paths = append(paths, e.Path)
	}

	if contains(paths, "skip.go") {
		t.Errorf("skip.go should be ignored, got: %v", paths)
	}
	if !contains(paths, "keep.go") {
		t.Errorf("keep.go should be present, got: %v", paths)
	}

	// С --no-gitignore skip.go возвращается
	entries, _ = Walk(Options{Root: root, UseGitignore: false})
	paths = nil
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	if !contains(paths, "skip.go") {
		t.Errorf("with UseGitignore=false skip.go should be present, got: %v", paths)
	}
}

func mustWrite(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
