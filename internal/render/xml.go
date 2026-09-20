// internal/render/xml.go

package render

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/it1ro/cake/internal/walker"
	"github.com/it1ro/cake/pkg/types"
)

// CDATA-маркеры.
//
// Close:  ]]]]><![CDATA[>  — 4 ']' + '>' + <![CDATA[ + '>'
//
//	Парсер видит: ]] (текст в текущем CDATA) + ]]> (закрытие)
//	+ <![CDATA[ (открытие нового) + > (текст в новом).
//	Поэтому содержимое между <![CDATA[ и этим маркером может
//	безопасно содержать ]]>.
//
// Escape: ]]]]]]><![CDATA[><![CDATA[>  — 6 ']' + два <![CDATA[
//
//	На это заменяется close-маркер, если он встретился в самом
//	содержимом. Замена добавляет два ']' и ещё один <![CDATA[,
//	чтобы наивный close-маркер внутри текста не закрыл блок.
const (
	cdataCloseMarker = "]]]]><![CDATA[>"
	cdataEscape      = "]]]]]]><![CDATA[><![CDATA[>"
)

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
	bw.WriteString("  " + cdataCloseMarker + "</tree>\n\n")

	if len(ctx.Omitted) > 0 {
		fmt.Fprintf(bw, "  <omitted count=\"%d\">\n", ctx.Dropped)
		for _, o := range ctx.Omitted {
			fmt.Fprintf(bw, "    <dir path=%q files=\"%d\" tokens=\"%d\"/>\n",
				o.Path, o.Files, o.Tokens)
		}
		bw.WriteString("  </omitted>\n\n")
	}

	for _, f := range ctx.Files {
		fmt.Fprintf(bw, "  <file path=%q lang=%q lines=\"%d\" bytes=\"%d\">\n",
			f.Entry.Path, f.Entry.Language, f.Lines, len(f.Content))
		bw.WriteString("  <![CDATA[\n")

		content := strings.ReplaceAll(string(f.Content), cdataCloseMarker, cdataEscape)
		bw.WriteString(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			bw.WriteByte('\n')
		}
		bw.WriteString("  " + cdataCloseMarker + "\n")
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
