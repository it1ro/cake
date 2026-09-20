package pipeline

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/it1ro/cake/pkg/types"
)

// osc52WarnLimit — эмпирический лимит xterm на OSC 52.
// README уже ссылается на 100 KB; здесь он же, чтобы предупреждение
// не расходилось с документацией.
const osc52WarnLimit = 100 * 1024

// writeSummary печатает однострочный отчёт о скопированном
// содержимом. Цвета включаются только если целевой writer —
// терминал; отключаются по NO_COLOR и TERM=dumb.
//
// Без лимита формат прежний:
//
//	✓ copied 42 files · ~18k tok · 234 KB → clipboard
//
// С заданным --context-limit добавляется отношение к эффективному
// потолку и сам лимит:
//
//	✓ copied 42 files · ≈171k / 180k (95%) · limit 200k · 234 KB → clipboard
//
// Плюс два предупреждения отдельными строками:
//   - при >= 90% потолка — «мало запаса на ответ»;
//   - при выводе > 100 KB — про лимит OSC 52 в некоторых терминалах.
func writeSummary(w io.Writer, opts Options, ctx types.Context, bytes int) {
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

	ceiling := effectiveCeiling(opts)
	var pct int
	if opts.ContextLimit > 0 && ceiling > 0 {
		pct = ctx.Tokens * 100 / ceiling
	}

	stats := []string{fmt.Sprintf("%d files", len(ctx.Files))}
	if ceiling > 0 {
		stats = append(stats,
			fmt.Sprintf("≈%s / %s (%d%%)",
				humanTokensCompact(ctx.Tokens),
				humanTokensCompact(ceiling),
				pct),
			fmt.Sprintf("limit %s", humanTokensCompact(opts.ContextLimit)),
		)
	} else {
		stats = append(stats, humanTokens(ctx.Tokens))
	}
	stats = append(stats, humanBytes(bytes))
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

	if ceiling > 0 && pct >= 90 {
		warn := "⚠ мало запаса на ответ"
		if color {
			warn = cYellow + warn + cReset
		}
		fmt.Fprintln(w, warn)
	}
	if bytes > osc52WarnLimit {
		warn := fmt.Sprintf(
			"⚠ вывод больше %d KB — некоторые терминалы режут OSC 52; используйте -o file",
			osc52WarnLimit/1024,
		)
		if color {
			warn = cYellow + warn + cReset
		}
		fmt.Fprintln(w, warn)
	}
}

const (
	cReset  = "\x1b[0m"
	cDim    = "\x1b[2m"
	cBold   = "\x1b[1m"
	cGreen  = "\x1b[32m"
	cCyan   = "\x1b[36m"
	cYellow = "\x1b[33m"
)

// isTTY: go-isatty вместо os.ModeCharDevice.
//
// ModeCharDevice считает /dev/null и NUL терминалом, из-за чего
// цвета утекали в пайп, а на Windows NUL вёл себя иначе.
// IsCygwinTerminal нужен для mintty/Git Bash: там stdout —
// пайп, но пользователь сидит в терминале.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func humanTokens(n int) string {
	if n >= 10000 {
		return fmt.Sprintf("~%dk tok", n/1000)
	}
	return fmt.Sprintf("~%d tok", n)
}

// humanTokensCompact — для summary с лимитом: без «tok» и «~»,
// чтобы строка не распухала. 171000 → 171k, 3200 → 3.2k.
func humanTokensCompact(n int) string {
	if n >= 10000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	if n >= 1000 {
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
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
