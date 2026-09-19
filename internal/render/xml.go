package render

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/it1ro/cake/internal/walker"
	"github.com/it1ro/cake/pkg/types"
)

// XML рендерит Context в XML-подобный формат.
// Стриминг: пишем по одному файлу, не буферизуем весь вывод.
func XML(ctx types.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintf(bw, `<context project=%q root=%q files="%d" tokens="%d"`,
		ctx.Project, ctx.Root, len(ctx.Files), ctx.Tokens)
	if ctx.Dropped > 0 {
		fmt.Fprintf(bw, ` dropped="%d"`, ctx.Dropped)
	}
	bw.WriteString(">\n")

	entries := make([]types.FileEntry, len(ctx.Files))
	for i, f := range ctx.Files {
		entries[i] = f.Entry
	}

	bw.WriteString("  <tree><![CDATA[\n")
	bw.WriteString(indent(walker.Tree(entries), "    "))
	bw.WriteString("  ]]></tree>\n\n")

	for _, f := range ctx.Files {
		fmt.Fprintf(bw, "  <file path=%q lang=%q lines=\"%d\" bytes=\"%d\">\n",
			f.Entry.Path, f.Entry.Language, f.Lines, len(f.Content))
		bw.WriteString("  <![CDATA[\n")

		content := strings.ReplaceAll(string(f.Content), "]]>", "]]]]><![CDATA[>")
		bw.WriteString(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			bw.WriteByte('\n')
		}
		bw.WriteString("  ]]>\n")
		bw.WriteString("  </file>\n\n")
	}

	bw.WriteString("</context>\n")
	return nil
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}
