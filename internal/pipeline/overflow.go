package pipeline

import (
	"fmt"
	"io"
	"os"

	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/internal/report"
	"github.com/it1ro/cake/internal/tokens"
	"github.com/it1ro/cake/pkg/types"
)

// OverflowMode — что делать при переполнении лимита.
type OverflowMode int

const (
	// OverflowFail — отчёт в stderr, exit 3, вывода нет.
	// Дефолт при заданном --context-limit.
	OverflowFail OverflowMode = iota
	// OverflowDrop — обрезка до потолка, отчёт-краткое в stderr,
	// exit 0. Соответствует старому --budget.
	OverflowDrop
)

// OverflowError — типизированная ошибка «не влезли».
// main.go различает её через errors.As и выходит с кодом 3.
//
// ReportErr — сопутствующая ошибка записи --report-file.
// Она не заменяет основную семантику: «не влезли» важнее,
// чем «файл не записался». main.go печатает оба сообщения
// и всё равно возвращает 3.
type OverflowError struct {
	Report    *report.Report
	ReportErr error
}

func (e *OverflowError) Error() string {
	base := fmt.Sprintf("context overflow: ~%d > %d (limit %d, reserve %d)",
		e.Report.Estimate, e.Report.Ceiling,
		e.Report.Limit, e.Report.Reserve)
	if e.ReportErr != nil {
		return base + "; " + e.ReportErr.Error()
	}
	return base
}

// CheckInput — что известно о наборе.
type CheckInput struct {
	// ContentTokens — суммарные токены содержимого (без обвязки).
	// В dump — по Size; в clean — по обработанному Content.
	ContentTokens int
	// Exact — true, если это точное значение (а не оценка сверху).
	Exact bool
}

// effectiveCeiling возвращает потолок проверки: min(budget, limit − reserve).
// 0 означает «лимита нет».
func effectiveCeiling(opts Options) int {
	if opts.ContextLimit <= 0 {
		return 0
	}
	reserve := opts.Reserve
	if reserve <= 0 {
		reserve = tokens.DefaultReserve(opts.ContextLimit)
	}
	ceiling := opts.ContextLimit - reserve
	if opts.Budget > 0 && opts.Budget < ceiling {
		ceiling = opts.Budget
	}
	return ceiling
}

// Check проверяет лимит. Возвращает nil, если:
//   - ContextLimit == 0 (лимит отключён), или
//   - оценка <= потолка.
//
// Иначе — заполненный *report.Report.
func Check(opts Options, entries []types.FileEntry, in CheckInput) *report.Report {
	ceiling := effectiveCeiling(opts)
	if ceiling == 0 {
		return nil
	}

	format := opts.Format
	if format == "" {
		format = render.FormatXML
	}
	overheadBytes := render.Overhead(format, opts.Root, entries)
	overheadTok := (overheadBytes + 3) / 4

	estimate := in.ContentTokens + overheadTok
	if estimate <= ceiling {
		return nil
	}

	reserve := opts.Reserve
	if reserve <= 0 {
		reserve = tokens.DefaultReserve(opts.ContextLimit)
	}
	return report.Build(report.Params{
		Entries:        entries,
		OverheadTokens: overheadTok,
		Limit:          opts.ContextLimit,
		Reserve:        reserve,
		Ceiling:        ceiling,
		Exact:          in.Exact,
		Mode:           modeString(opts.Mode),
	})
}

func modeString(m Mode) string {
	if m == ModeClean {
		return "clean"
	}
	return "dump"
}

// reportWriter — куда печатать текстовый отчёт (nil → stderr).
func (o Options) reportWriter() io.Writer {
	if o.Report != nil {
		return o.Report
	}
	return os.Stderr
}

// handleOverflow обрабатывает переполнение. Возвращает nil при
// OverflowDrop (модифицирует opts.Budget, чтобы бюджетный фильтр
// обрезал набор) и *OverflowError при OverflowFail.
//
// Ошибка записи --report-file не глотается: если путь указывает
// в несуществующую директорию, пользователь узнает об этом
// сразу. При OverflowFail она кладётся в ReportErr, чтобы main
// не потерял exit-код 3 — «не влезли» остаётся главным фактом,
// «файл не записался» — сопутствующим.
func handleOverflow(opts *Options, r *report.Report) error {
	if opts.OnOverflow == OverflowDrop {
		fmt.Fprintf(opts.reportWriter(),
			"⚠ контекст обрезан до потолка %d tok (лимит %d, резерв %d)\n",
			r.Ceiling, r.Limit, r.Reserve)
		if opts.ReportFile != "" {
			if err := r.WriteJSON(opts.ReportFile); err != nil {
				return fmt.Errorf("report-file %s: %w", opts.ReportFile, err)
			}
		}
		// Жёсткий бюджет: применяется в бюджетном фильтре ниже.
		opts.Budget = r.Ceiling
		return nil
	}
	r.Write(opts.reportWriter())
	if opts.ReportFile != "" {
		if err := r.WriteJSON(opts.ReportFile); err != nil {
			return &OverflowError{
				Report:    r,
				ReportErr: fmt.Errorf("report-file %s: %w", opts.ReportFile, err),
			}
		}
	}
	return &OverflowError{Report: r}
}
