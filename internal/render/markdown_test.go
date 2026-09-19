package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/it1ro/cake/pkg/types"
)

func TestMarkdown_Basic(t *testing.T) {
	ctx := types.Context{
		Project: "cake",
		Tokens:  1234,
		Files: []types.ProcessedFile{
			{
				Entry:   types.FileEntry{Path: "a.go", Language: "go"},
				Content: []byte("package a\n"), Lines: 1,
			},
		},
	}
	var buf bytes.Buffer
	if err := Markdown(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"# Context: cake",
		"Files: 1 · Tokens: ~1234",
		"## Tree",
		"## `a.go`",
		"```go",
		"package a",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// Содержимое с ``` не должно рвать fence.
func TestMarkdown_EscapesFence(t *testing.T) {
	ctx := types.Context{
		Project: "p",
		Files: []types.ProcessedFile{
			{
				Entry:   types.FileEntry{Path: "doc.md", Language: "markdown"},
				Content: []byte("text\n```\ninner\n```\n"),
			},
		},
	}
	var buf bytes.Buffer
	if err := Markdown(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// внешний fence открыт один раз, закрыт один раз — считаем
	// только строки, начинающиеся с ``` на позиции 0.
	fences := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "```") {
			fences++
		}
	}
	// открытие ``` + язык для a.go, закрытие, открытие ```markdown,
	// закрытие — минимум 4, но внутренние ``` заменены, поэтому
	// ровно 4.
	if fences != 4 {
		t.Errorf("unexpected fence count %d\n--- got ---\n%s", fences, out)
	}
	if strings.Contains(out, "\n```\ninner") {
		t.Errorf("inner fence not escaped:\n%s", out)
	}
}

func TestMarkdown_Dropped(t *testing.T) {
	ctx := types.Context{Project: "p", Tokens: 10, Dropped: 3}
	var buf bytes.Buffer
	if err := Markdown(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Dropped: 3") {
		t.Errorf("dropped count not shown: %s", buf.String())
	}
}
