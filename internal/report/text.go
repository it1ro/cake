package report

import (
	"fmt"
	"io"
	"strings"
)

// Write печатает отчёт в текстовом виде (см. review §4.4).
// Единица веса — токены, обвязка формата включена в estimate.
func (r *Report) Write(w io.Writer) {
	approx := "≈"
	if r.Exact {
		approx = "≈" // даже "точное" значение — эвристика len/4
	}

	fmt.Fprintf(w, "✗ не влезаем: %s%dk tok при потолке %dk (лимит %dk, резерв %dk)\n",
		approx, r.Estimate/1000, r.Ceiling/1000, r.Limit/1000, r.Reserve/1000)
	if !r.Exact {
		fmt.Fprintln(w, "  [оценка сверху: по Size, без чтения содержимого]")
	}
	fmt.Fprintln(w)

	if len(r.Dirs) > 0 {
		fmt.Fprintln(w, "Где вес:")
		for _, b := range r.Dirs {
			pct := 0
			if r.Estimate > 0 {
				pct = b.Tokens * 100 / r.Estimate
			}
			label := b.Key
			if b.Key == "./" {
				label = "./"
			}
			fmt.Fprintf(w, "  %-20s %5dk  %2d%%   (%d файлов)\n",
				label, b.Tokens/1000, pct, b.Files)
		}
		fmt.Fprintln(w)
	}

	if len(r.Exts) > 0 {
		var parts []string
		for _, b := range r.Exts {
			parts = append(parts, fmt.Sprintf("%s %dk", b.Key, b.Tokens/1000))
		}
		fmt.Fprintf(w, "Расширения:  %s\n\n", strings.Join(parts, " · "))
	}

	if len(r.Measures) == 0 {
		fmt.Fprintln(w, "Готовых мер нет — попробуйте `cake pick .` для ручного выбора.")
		return
	}

	anyFits := false
	for _, m := range r.Measures {
		if m.Fits {
			anyFits = true
			break
		}
	}
	if anyFits {
		fmt.Fprintln(w, "Что сделать (набор влезает):")
	} else {
		fmt.Fprintln(w, "Что сделать (ни одна мера не влезает одна):")
	}
	for _, m := range r.Measures {
		flags := strings.Join(m.Flags, " ")
		if flags == "" {
			flags = "--mode " + m.Label
		}
		mark := "✗ не влезает"
		if m.Fits {
			mark = "✓ влезает"
		}
		fmt.Fprintf(w, "  %-40s → %s%dk  %s\n",
			flags, approx, m.After/1000, mark)
	}
	if !anyFits {
		fmt.Fprintln(w, "  cake pick .                              → вручную")
	}
}
