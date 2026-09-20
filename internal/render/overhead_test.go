package render

import (
	"bytes"
	"testing"

	"github.com/it1ro/cake/pkg/types"
)

// Overhead не должен расходиться с реальным выводом (без содержимого)
// больше, чем на допуск по числу файлов. Тест-сверка: правка шаблона
// рендера сразу видна здесь.
func TestOverhead_MatchesRender(t *testing.T) {
	entries := []types.FileEntry{
		{Path: "a.go", Language: "go"},
		{Path: "cmd/b.go", Language: "go"},
		{Path: "internal/x/y.go", Language: "go"},
	}
	content := []byte("package a\n")

	for _, f := range []Format{FormatXML, FormatMarkdown, FormatPlain} {
		pf := make([]types.ProcessedFile, len(entries))
		for i, e := range entries {
			pf[i] = types.ProcessedFile{Entry: e, Content: content, Lines: 1}
		}
		var buf bytes.Buffer
		if err := Render(types.Context{Project: "p", Root: "p", Files: pf}, f, &buf); err != nil {
			t.Fatal(err)
		}
		want := buf.Len() - len(content)*len(entries)
		got := Overhead(f, "p", entries)

		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		// допуск: 12 байт на файл + 16 байт общего оверхеда
		if diff > 12*len(entries)+16 {
			t.Errorf("%s: Overhead=%d, реальная обвязка=%d (разница %d)",
				f, got, want, diff)
		}
	}
}

func TestOverhead_Grows(t *testing.T) {
	one := []types.FileEntry{{Path: "a.go"}}
	two := []types.FileEntry{{Path: "a.go"}, {Path: "b.go"}}
	if Overhead(FormatXML, "p", two) <= Overhead(FormatXML, "p", one) {
		t.Error("обвязка должна расти с числом файлов")
	}
}
