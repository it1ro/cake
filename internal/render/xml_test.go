package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/it1ro/cake/pkg/types"
)

func TestXML_Basic(t *testing.T) {
	ctx := types.Context{
		Project: "test",
		Root:    "/tmp/test",
		Files: []types.ProcessedFile{
			{
				Entry:   types.FileEntry{Path: "a.go", Language: "go"},
				Content: []byte("package a\n"),
				Lines:   1,
			},
		},
	}
	var buf bytes.Buffer
	if err := XML(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		`<context project="test"`,
		`files="1"`,
		`<file path="a.go"`,
		`lang="go"`,
		`<![CDATA[`,
		"package a",
		"</context>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// Главный кейс: если в исходнике встречается ]]>,
// наивный CDATA сломается. Проверяем, что мы его экранируем.
func TestXML_EscapesCDATA(t *testing.T) {
	ctx := types.Context{
		Project: "test",
		Files: []types.ProcessedFile{
			{
				Entry:   types.FileEntry{Path: "a.go", Language: "go"},
				Content: []byte(`s := "]]>"` + "\n"),
			},
		},
	}
	var buf bytes.Buffer
	if err := XML(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Наш эскейп: ]]> → ]]]]><![CDATA[>
	if !strings.Contains(out, "]]]]><![CDATA[>") {
		t.Errorf("CDATA sequence not escaped\n--- got ---\n%s", out)
	}
}

// Все атрибуты должны быть в кавычках — иначе XML невалиден
// и часть парсеров отвергает документ.
func TestXML_AllAttributesQuoted(t *testing.T) {
	ctx := types.Context{
		Project: "p",
		Root:    "/r",
		Files: []types.ProcessedFile{
			{
				Entry:   types.FileEntry{Path: "a.go", Language: "go"},
				Content: []byte("package a\n"),
				Lines:   1,
			},
		},
	}
	var buf bytes.Buffer
	if err := XML(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	// Простой предикат: каждое `=` в тегах должно сопровождаться `"`.
	for _, line := range strings.Split(buf.String(), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "<") {
			continue
		}
		if strings.Contains(trimmed, "=") && !strings.Contains(trimmed, `="`) {
			t.Errorf("unquoted attribute in line: %s", line)
		}
	}
}

// Без cdataCloseMarker в содержимом рендер не должен добавлять
// экранирующий двойной CDATA. Считаем закрывающие маркеры: для
// одного <tree> и одного <file> их ровно 2.
func TestXML_NoEscapingWhenNotNeeded(t *testing.T) {
	ctx := types.Context{
		Project: "test",
		Files: []types.ProcessedFile{
			{
				Entry:   types.FileEntry{Path: "a.go", Language: "go"},
				Content: []byte("package a\n"),
			},
		},
	}
	var buf bytes.Buffer
	if err := XML(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Закрывающих маркеров ровно 2: для <tree> и для <file>.
	if got := strings.Count(out, cdataCloseMarker); got != 2 {
		t.Errorf("want 2 CDATA close markers, got %d:\n%s", got, out)
	}
	// Экранированного варианта быть не должно. cdataEscape длиннее
	// cdataCloseMarker, поэтому его наличие — точный признак, что
	// применили escape к содержимому.
	if strings.Contains(out, cdataEscape) {
		t.Errorf("unexpected CDATA escape:\n%s", out)
	}
}

// Константы — не магические строки: close должен начинаться
// ровно с четырёх ']', escape — ровно с шести. Ошибка в один ']'
// ломает XML-парсер у LLM, поэтому фиксируем форму отдельно.
func TestCDATAMarkerShape(t *testing.T) {
	if !strings.HasPrefix(cdataCloseMarker, "]]]]>") {
		t.Errorf("cdataCloseMarker должен начинаться с ']]]]>', got %q",
			cdataCloseMarker)
	}
	if strings.HasPrefix(cdataCloseMarker, "]]]]]>") {
		t.Errorf("cdataCloseMarker: лишний ']' в начале: %q", cdataCloseMarker)
	}
	if !strings.HasPrefix(cdataEscape, "]]]]]]>") {
		t.Errorf("cdataEscape должен начинаться с ']]]]]]>', got %q", cdataEscape)
	}
	// Escape содержит close-маркер начиная с позиции 2 — это и есть
	// механизм: «сдвинули на два ']', чтобы наивный close не сработал».
	if !strings.Contains(cdataEscape, cdataCloseMarker) {
		t.Errorf("cdataEscape должен содержать cdataCloseMarker:\n  escape=%q\n  marker=%q",
			cdataEscape, cdataCloseMarker)
	}
}
