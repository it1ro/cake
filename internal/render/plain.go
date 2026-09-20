package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/it1ro/cake/internal/walker"
	"github.com/it1ro/cake/pkg/types"
)

// Plain рендерит Context без разметки — для отладки и для
// случаев, когда теги мешают (например, диффы).
func Plain(ctx types.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintf(bw, "project: %s\n", ctx.Project)
	fmt.Fprintf(bw, "files: %d, tokens: ~%d", len(ctx.Files), ctx.Tokens)
	if ctx.Dropped > 0 {
		fmt.Fprintf(bw, ", dropped: %d", ctx.Dropped)
	}
	bw.WriteByte('\n')

	entries := make([]types.FileEntry, len(ctx.Files))
	for i, f := range ctx.Files {
		entries[i] = f.Entry
	}
	if len(entries) > 0 {
		bw.WriteString("\ntree:\n")
		bw.WriteString(walker.Tree(entries))
	}

	if len(ctx.Omitted) > 0 {
		bw.WriteString("\nomitted:\n")
		for _, o := range ctx.Omitted {
			fmt.Fprintf(bw, "  %s — %d files (~%d tok)\n",
				o.Path, o.Files, o.Tokens)
		}
	}

	for _, f := range ctx.Files {
		bw.WriteString("\n")
		fmt.Fprintf(bw, "────── %s (%s) ──────\n", f.Entry.Path, f.Entry.Language)
		bw.Write(f.Content)
		if len(f.Content) > 0 && f.Content[len(f.Content)-1] != '\n' {
			bw.WriteByte('\n')
		}
	}
	return nil
}
