package render

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/it1ro/cake/internal/walker"
	"github.com/it1ro/cake/pkg/types"
)

// Markdown рендерит Context как markdown-документ.
// Удобно вставлять прямо в чат: заголовки сворачиваются в TOC,
// code-блоки подсвечиваются.
func Markdown(ctx types.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintf(bw, "# Context: %s\n\n", ctx.Project)
	fmt.Fprintf(bw, "_Files: %d · Tokens: ~%d", len(ctx.Files), ctx.Tokens)
	if ctx.Dropped > 0 {
		fmt.Fprintf(bw, " · Dropped: %d", ctx.Dropped)
	}
	bw.WriteString("_\n\n")

	entries := make([]types.FileEntry, len(ctx.Files))
	for i, f := range ctx.Files {
		entries[i] = f.Entry
	}
	bw.WriteString("## Tree\n\n```\n")
	bw.WriteString(walker.Tree(entries))
	bw.WriteString("```\n\n")

	for _, f := range ctx.Files {
		lang := f.Entry.Language
		if lang == "" {
			lang = "text"
		}
		fmt.Fprintf(bw, "## `%s`\n\n", f.Entry.Path)
		fmt.Fprintf(bw, "_%s · %d lines · %d bytes_\n\n", lang, f.Lines, len(f.Content))
		fmt.Fprintf(bw, "```%s\n", lang)

		// Внутри fence нельзя допускать строку ``` — иначе блок
		// закроется раньше времени. Заменяем на ‛‛‛ (U+201B).
		content := strings.ReplaceAll(string(f.Content), "```", "‛‛‛")
		bw.WriteString(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			bw.WriteByte('\n')
		}
		bw.WriteString("```\n\n")
	}
	return nil
}
