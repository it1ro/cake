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

// Обратный тест: файл без ]]> не должен получать лишних эскейпов.
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
	if strings.Contains(buf.String(), "]]]]><![CDATA[>") {
		t.Errorf("unexpected CDATA escape:\n%s", buf.String())
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
