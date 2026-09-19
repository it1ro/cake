package pipeline

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/it1ro/cake/pkg/types"
)

// writeSummary печатает однострочный отчёт о скопированном
// содержимом. Цвета включаются только если целевой writer —
// терминал; отключаются по NO_COLOR и TERM=dumb.
//
// Одна строка, а не блок: терминал не «уезжает», grep по логам
// не ломается. Формат:
//
//	✓ copied 42 files · ~18k tok · 234 KB → clipboard
func writeSummary(w io.Writer, ctx types.Context, bytes int) {
	if w == nil {
		w = os.Stderr
	}

	color := isTTY(w) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"

	var b strings.Builder

	if color {
		b.WriteString(cGreen + "✓" + cReset + " " + cBold + "copied" + cReset)
	} else {
		b.WriteString("✓ copied")
	}
	b.WriteString(" ")

	stats := []string{
		fmt.Sprintf("%d files", len(ctx.Files)),
		humanTokens(ctx.Tokens),
		humanBytes(bytes),
	}
	if ctx.Dropped > 0 {
		stats = append(stats, fmt.Sprintf("dropped %d", ctx.Dropped))
	}

	if color {
		b.WriteString(cDim)
	}
	b.WriteString(strings.Join(stats, " · "))
	if color {
		b.WriteString(cReset)
	}

	b.WriteString(" ")
	if color {
		b.WriteString(cCyan)
	}
	b.WriteString("→ clipboard")
	if color {
		b.WriteString(cReset)
	}

	fmt.Fprintln(w, b.String())
}

const (
	cReset = "\x1b[0m"
	cDim   = "\x1b[2m"
	cBold  = "\x1b[1m"
	cGreen = "\x1b[32m"
	cCyan  = "\x1b[36m"
)

// isTTY сообщает, является ли w символьным устройством.
// Без новых зависимостей: os.Stderr — char device, файл — нет.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func humanTokens(n int) string {
	if n >= 10000 {
		return fmt.Sprintf("~%dk tok", n/1000)
	}
	return fmt.Sprintf("~%d tok", n)
}

func humanBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
