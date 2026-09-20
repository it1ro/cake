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
	if !strings.Contains(out, cdataEscape) {
		t.Errorf("CDATA sequence not escaped\n--- got ---\n%s", out)
	}
	// Сырое `"]]>"` (с кавычками из исходника) в выводе остаться
	// не должно — оно должно было превратиться в
	// `"]]]]><![CDATA[>"`.
	if strings.Contains(out, `"]]>"`) {
		t.Errorf("сырое ]]> осталось в содержимом:\n%s", out)
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

// Без `]]>` в содержимом рендер не должен добавлять экранирующий
// двойной CDATA. Считаем закрывающие маркеры: для одного <tree>
// и одного <file> их ровно 2.
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
	// Экранированного варианта быть не должно: содержимое не
	// содержит `]]>`, поэтому замен не было.
	if strings.Contains(out, cdataEscape) {
		t.Errorf("unexpected CDATA escape:\n%s", out)
	}
}

// Константы — не магические строки. cdataCloseMarker — это
// стандартный `]]>`; cdataEscape — безопасная замена, которая
// при парсинге даёт обратно `]]>`. Ошибка в один `]` ломает
// XML-парсер у LLM, поэтому фиксируем форму отдельно.
func TestCDATAMarkerShape(t *testing.T) {
	if cdataCloseMarker != "]]>" {
		t.Errorf("cdataCloseMarker = %q, want %q",
			cdataCloseMarker, "]]>")
	}
	if cdataEscape != "]]]]><![CDATA[>" {
		t.Errorf("cdataEscape = %q, want %q",
			cdataEscape, "]]]]><![CDATA[>")
	}
	// Отдельно фиксируем число ведущих `]` в escape: ровно 4.
	// Ровно 4 — потому что парсер должен увидеть `]]` + `]]>`,
	// то есть 2 `]` в первой CDATA и `]]>` как закрытие. Если
	// поставить 6, парсер срежет лишние как часть закрытия, и
	// round-trip сломается.
	beforeGT := cdataEscape[:strings.Index(cdataEscape, ">")]
	if n := strings.Count(beforeGT, "]"); n != 4 {
		t.Errorf("cdataEscape: want 4 ведущих ']', got %d (%q)",
			n, cdataEscape)
	}
}
