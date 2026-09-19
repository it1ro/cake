package golang

// Package golang реализует процессор для .go файлов:
// strip комментариев с сохранением валидности кода.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"

	"github.com/it1ro/cake/internal/processor"
)

func init() {
	processor.Register(Go{})
}

// Go — процессор для .go файлов.
type Go struct{}

func (Go) Supports(path string) bool { return strings.HasSuffix(path, ".go") }
func (Go) Language(string) string    { return "go" }

// Process парсит файл, убирает комментарии и печатает обратно.
//
// Сохраняются всегда:
//   - build-constraints: //go:build, // +build
//   - директивы: //go:generate, //go:embed, //go:noinline, //line …
//
// Сохраняются при opts.KeepDoc:
//   - doc-комментарии перед package, type, func, var, const
//
// При ошибке парсинга возвращает исходник без ошибки — пайплайн
// не должен падать из-за одного битого файла (например, файл
// с build-тегом под другой GOOS).
func (Go) Process(path string, src []byte, opts processor.Options) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return src, nil
	}

	if !opts.KeepDoc {
		f.Doc = nil
	}

	var docSet map[*ast.CommentGroup]bool
	if opts.KeepDoc {
		docSet = collectDocComments(f)
	}

	// Гарантируем non-nil f.Comments: если оставить nil,
	// go/printer переключится в режим useNodeComments и начнёт
	// печатать комментарии из .Doc-полей AST, обойдя наш фильтр.
	// Подробности — в конце раздела.
	if f.Comments == nil {
		f.Comments = []*ast.CommentGroup{}
	}
	kept := f.Comments[:0]
	for _, cg := range f.Comments {
		if keepGroup(cg, docSet) {
			kept = append(kept, cg)
		}
	}
	f.Comments = kept

	var buf bytes.Buffer
	cfg := printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8, // как в gofmt
	}
	if err := cfg.Fprint(&buf, fset, f); err != nil {
		return src, err
	}
	return buf.Bytes(), nil
}

// keepGroup решает судьбу одной группы комментариев.
func keepGroup(cg *ast.CommentGroup, docSet map[*ast.CommentGroup]bool) bool {
	for _, c := range cg.List {
		t := c.Text
		// Директивы компилятора — сохраняем всегда.
		if strings.HasPrefix(t, "//go:") ||
			strings.HasPrefix(t, "//line ") {
			return true
		}
		// Старый синтаксис build-constraints.
		if strings.HasPrefix(t, "// +build") ||
			strings.HasPrefix(t, "//+build") {
			return true
		}
	}
	if docSet != nil && docSet[cg] {
		return true
	}
	return false
}

// collectDocComments возвращает множество групп, привязанных
// к объявлениям верхнего уровня как .Doc.
func collectDocComments(f *ast.File) map[*ast.CommentGroup]bool {
	set := make(map[*ast.CommentGroup]bool)
	if f.Doc != nil {
		set[f.Doc] = true
	}
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.GenDecl:
			if decl.Doc != nil {
				set[decl.Doc] = true
			}
		case *ast.FuncDecl:
			if decl.Doc != nil {
				set[decl.Doc] = true
			}
		}
	}
	return set
}
