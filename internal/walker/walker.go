package walker

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/it1ro/cake/internal/gitignore"
	"github.com/it1ro/cake/internal/processor"
	"github.com/it1ro/cake/pkg/types"
)

type Options struct {
	Root         string
	Includes     []string
	Excludes     []string
	MaxSize      int64
	UseGitignore bool
}

func Walk(opts Options) ([]types.FileEntry, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}

	var entries []types.FileEntry
	var gm *gitignore.Matcher
	if opts.UseGitignore {
		gm = gitignore.New(root)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}

		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}

			// Сначала проверяем, не игнорируется ли сама директория
			// по уже накопленным (родительским) правилам.
			if gm != nil && rel != "" && gm.Ignore(rel, true) {
				return filepath.SkipDir
			}

			// Только пустив директорию внутрь, читаем её .gitignore —
			// его правила применяются к содержимому, не к ней самой.
			if gm != nil {
				if err := gm.Load(path); err != nil {
					return err
				}
			}
			return nil
		}

		if gm != nil && gm.Ignore(rel, false) {
			return nil
		}

		if !matches(rel, opts.Includes, opts.Excludes) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if opts.MaxSize > 0 && info.Size() > opts.MaxSize {
			return nil
		}
		if isBinary(path) {
			return nil
		}

		entries = append(entries, types.FileEntry{
			Path:     rel,
			AbsPath:  path,
			Size:     info.Size(),
			Language: processor.Language(path),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func matches(rel string, includes, excludes []string) bool {
	for _, ex := range excludes {
		if ok, _ := doublestar.Match(ex, rel); ok {
			return false
		}
	}
	if len(includes) == 0 {
		return true
	}
	for _, inc := range includes {
		if ok, _ := doublestar.Match(inc, rel); ok {
			return true
		}
	}
	return false
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// Tree рисует ASCII-дерево из отсортированного списка путей.
func Tree(entries []types.FileEntry) string {
	if len(entries) == 0 {
		return ""
	}

	root := newTreeNode("")
	for _, e := range entries {
		root.insert(strings.Split(e.Path, "/"))
	}

	var sb strings.Builder
	root.render("", &sb)
	return sb.String()
}

type treeNode struct {
	name     string
	isFile   bool
	children []*treeNode
	index    map[string]*treeNode
}

func newTreeNode(name string) *treeNode {
	return &treeNode{name: name, index: make(map[string]*treeNode)}
}

func (n *treeNode) insert(parts []string) {
	if len(parts) == 0 {
		return
	}
	name := parts[0]
	isFile := len(parts) == 1

	child, ok := n.index[name]
	if !ok {
		child = newTreeNode(name)
		child.isFile = isFile
		n.index[name] = child
		n.children = append(n.children, child)
	} else if isFile {
		child.isFile = true
	}
	child.insert(parts[1:])
}

func (n *treeNode) render(prefix string, sb *strings.Builder) {
	for i, c := range n.children {
		isLast := i == len(n.children)-1

		var connector, nextPrefix string
		if isLast {
			connector = "└── "
			nextPrefix = prefix + "    "
		} else {
			connector = "├── "
			nextPrefix = prefix + "│   "
		}

		sb.WriteString(prefix)
		sb.WriteString(connector)
		sb.WriteString(c.name)
		if !c.isFile {
			sb.WriteString("/")
		}
		sb.WriteByte('\n')

		c.render(nextPrefix, sb)
	}
}
