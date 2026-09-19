package tui

import (
	"sort"
	"strings"

	"github.com/it1ro/cake/pkg/types"
)

// Node — узел дерева файлов. Корень — сентинел с Path="" и Name="".
type Node struct {
	Name     string
	Path     string // относительный путь от корня; "" для сентинела
	IsDir    bool
	Children []*Node
	Parent   *Node
	Expanded bool // для директорий; на листьях не значимо

	// hasMatch — внутреннее состояние фильтрации: узел нужно
	// показать (сам совпал или у него есть совпавший потомок).
	hasMatch bool
}

// flatItem — одна строка в развёрнутом дереве.
type flatItem struct {
	node   *Node
	prefix string // "│   ├── " и т.п.; без имени и без mark
}

// buildTree строит дерево из плоского списка путей.
// Вход отсортирован walker'ом, но дерево всё равно сортирует
// потомков само: dirs first, alpha.
func buildTree(entries []types.FileEntry) *Node {
	root := &Node{Path: "", IsDir: true, Expanded: true}
	for _, e := range entries {
		parts := strings.Split(e.Path, "/")
		cur := root
		for i, name := range parts {
			isLast := i == len(parts)-1
			full := strings.Join(parts[:i+1], "/")

			child := findChild(cur, name)
			if child == nil {
				child = &Node{
					Name:     name,
					Path:     full,
					IsDir:    !isLast,
					Parent:   cur,
					Expanded: true,
				}
				cur.Children = append(cur.Children, child)
			}
			cur = child
		}
	}
	sortTree(root)
	return root
}

func findChild(n *Node, name string) *Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// sortTree: директории перед файлами, внутри групп — по имени.
// Совпадает с walker.Tree и tree(1).
func sortTree(n *Node) {
	sort.Slice(n.Children, func(i, j int) bool {
		a, b := n.Children[i], n.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return a.Name < b.Name
	})
	for _, c := range n.Children {
		sortTree(c)
	}
}

// flatten разворачивает дерево в список видимых строк с учётом
// текущего фильтра. Пустой фильтр — все раскрытые узлы. Непустой —
// только ветки, содержащие совпадение; такие ветки считаются
// раскрытыми (иначе совпадения внутри были бы скрыты).
//
// Фильтр в дереве — подстрочный (case-insensitive), не fuzzy:
// пользователь ожидает «скрыть всё, что не подходит», а не
// переупорядочивание по релевантности.
func flatten(root *Node, filter string) []flatItem {
	lf := strings.ToLower(filter)
	markMatches(root, lf)
	forceExpand := filter != ""

	var out []flatItem
	flattenWalk(root, "", forceExpand, &out)
	return out
}

func markMatches(n *Node, lowerFilter string) bool {
	if lowerFilter == "" {
		n.hasMatch = true
		for _, c := range n.Children {
			markMatches(c, lowerFilter)
		}
		return true
	}
	self := strings.Contains(strings.ToLower(n.Path), lowerFilter)
	any := self
	for _, c := range n.Children {
		if markMatches(c, lowerFilter) {
			any = true
		}
	}
	n.hasMatch = any
	return any
}

func flattenWalk(n *Node, prefix string, forceExpand bool, out *[]flatItem) {
	visible := make([]*Node, 0, len(n.Children))
	for _, c := range n.Children {
		if c.hasMatch {
			visible = append(visible, c)
		}
	}
	for i, c := range visible {
		isLast := i == len(visible)-1
		var connector, cont string
		if isLast {
			connector = "└── "
			cont = "    "
		} else {
			connector = "├── "
			cont = "│   "
		}

		*out = append(*out, flatItem{node: c, prefix: prefix + connector})

		if c.IsDir && (c.Expanded || forceExpand) {
			flattenWalk(c, prefix+cont, forceExpand, out)
		}
	}
}

// fileDescendants возвращает пути всех файлов под узлом (или сам
// путь, если узел — файл). Для отката выбора в space/d.
func (n *Node) fileDescendants() []string {
	if !n.IsDir {
		return []string{n.Path}
	}
	var out []string
	var walk func(*Node)
	walk = func(x *Node) {
		for _, c := range x.Children {
			if c.IsDir {
				walk(c)
			} else {
				out = append(out, c.Path)
			}
		}
	}
	walk(n)
	return out
}

// selectState возвращает «сколько из скольки» выбрано под узлом.
// Используется для отрисовки [x]/[-]/[ ].
func selectState(n *Node, sel map[string]bool) (selCount, total int) {
	files := n.fileDescendants()
	total = len(files)
	for _, p := range files {
		if sel[p] {
			selCount++
		}
	}
	return selCount, total
}
