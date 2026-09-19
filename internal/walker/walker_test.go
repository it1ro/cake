package walker

import (
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
