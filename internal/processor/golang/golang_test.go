package golang

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/it1ro/cake/internal/processor"
)

func TestGoProcess(t *testing.T) {
	tests := []struct {
		name            string
		src             string
		keepDoc         bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "removes_line_comments",
			src: `package main

// standalone comment
func main() {
	// inner comment
	println("hi") // trailing
}
`,
			wantNotContains: []string{"standalone", "inner comment", "trailing"},
			wantContains:    []string{"package main", "func main()", `println("hi")`},
		},
		{
			name: "removes_block_comments",
			src: `package main

/*
block comment
spanning lines
*/
func main() {}
`,
			wantNotContains: []string{"block comment", "spanning lines"},
			wantContains:    []string{"package main", "func main()"},
		},
		{
			name: "removes_doc_by_default",
			src: `package main

// Foo does foo.
func Foo() {}

// Bar is a type.
type Bar int
`,
			wantNotContains: []string{"Foo does foo", "Bar is a type"},
			wantContains:    []string{"func Foo()", "type Bar int"},
		},
		{
			name: "keeps_doc_with_flag",
			src: `package main

// Foo does foo.
func Foo() {}
`,
			keepDoc:      true,
			wantContains: []string{"Foo does foo"},
		},
		{
			name: "keeps_go_build_directive",
			src: `//go:build linux
// +build linux

package main

func main() {}
`,
			wantContains: []string{"//go:build linux", "// +build linux"},
		},
		{
			name: "keeps_go_generate_directive",
			src: `package main

//go:generate stringer -type=Foo
type Foo int
`,
			wantContains: []string{"//go:generate stringer -type=Foo"},
		},
		{
			name: "keeps_go_embed_directive",
			src: `package main

import _ "embed"

//go:embed hello.txt
var hello string
`,
			wantContains: []string{"//go:embed hello.txt"},
		},
		{
			name: "removes_nolint",
			src: `package main

func f() {} //nolint:unused

// nolint is not a compiler directive, only a linter hint
`,
			wantNotContains: []string{"nolint"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Go{}.Process("x.go", []byte(tc.src), processor.Options{KeepDoc: tc.keepDoc})
			if err != nil {
				t.Fatalf("Process returned error: %v", err)
			}
			got := string(out)
			for _, s := range tc.wantContains {
				if !strings.Contains(got, s) {
					t.Errorf("want output to contain %q\n--- got ---\n%s", s, got)
				}
			}
			for _, s := range tc.wantNotContains {
				if strings.Contains(got, s) {
					t.Errorf("want output NOT to contain %q\n--- got ---\n%s", s, got)
				}
			}
		})
	}
}

// На malformed-входе процессор не падает, а возвращает исходник.
// Иначе один битый файл (например, под чужой GOOS) валит весь дамп.
func TestGoProcess_Malformed_ReturnsOriginal(t *testing.T) {
	src := []byte("package main\nfunc broken(\n")
	out, err := Go{}.Process("x.go", src, processor.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string(src) {
		t.Errorf("want original source untouched, got:\n%s", out)
	}
}

// Выход должен оставаться синтаксически валидным Go —
// иначе смысл утилиты теряется.
func TestGoProcess_OutputIsParseable(t *testing.T) {
	inputs := []string{
		`package main

// comment
func main() {
	// inner
	println("hi")
}
`,
		`//go:build linux

package main

var x = 1
`,
		`package main

/*
multi
line
*/
func f() {}
`,
	}
	for i, src := range inputs {
		out, err := Go{}.Process("x.go", []byte(src), processor.Options{})
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "x.go", out, parser.AllErrors); err != nil {
			t.Errorf("case %d: output not parseable: %v\n---\n%s", i, err, out)
		}
	}
}

func TestGoSupports(t *testing.T) {
	p := Go{}
	if !p.Supports("main.go") {
		t.Error("should support .go")
	}
	if p.Supports("main.rs") {
		t.Error("should not support .rs")
	}
}

// Проверяем, что init() зарегистрировал процессор в реестре.
func TestGoIsRegistered(t *testing.T) {
	p := processor.For("main.go")
	if _, ok := p.(Go); !ok {
		t.Fatalf("Go processor not registered; got %T", p)
	}
}
