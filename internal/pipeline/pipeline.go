package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/it1ro/cake/internal/clipboard"
	"github.com/it1ro/cake/internal/processor"
	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/internal/tokens"
	"github.com/it1ro/cake/internal/walker"
	"github.com/it1ro/cake/pkg/types"
)

type Mode int

const (
	ModeDump Mode = iota
	ModeClean
)

type Options struct {
	Root         string
	Includes     []string
	Excludes     []string
	MaxSize      int64
	Output       string
	Mode         Mode
	Format       render.Format
	KeepDoc      bool
	UseGitignore bool
	Budget       int
	Clipboard    bool

	// Лимит контекста (review §D4, §D11). ContextLimit == 0 —
	// проверок нет. Reserve < 0 означает «посчитать автоматически»;
	// Reserve == 0 при явно заданном ContextLimit — тоже.
	ContextLimit int
	Reserve      int
	OnOverflow   OverflowMode

	// Report — куда печатать диагностику переполнения.
	// nil → stderr. ReportFile — путь к JSON-версии.
	Report     io.Writer
	ReportFile string

	// Summary — куда писать краткий отчёт после clipboard.
	Summary io.Writer
}

// Plan — обход ФС и применение фильтров. См. комментарий в
// walker.Options.
func Plan(opts Options) ([]types.FileEntry, error) {
	return walker.Walk(walker.Options{
		Root:         opts.Root,
		Includes:     opts.Includes,
		Excludes:     opts.Excludes,
		MaxSize:      opts.MaxSize,
		UseGitignore: opts.UseGitignore,
	})
}

// Run — Plan + RunWith. Точка входа для CLI-команд.
func Run(opts Options) error {
	files, err := Plan(opts)
	if err != nil {
		return err
	}
	return RunWith(opts, files)
}

// RunWith — обработка и рендер для указанного набора файлов.
//
// Порядок:
//  1. Warn: если --budget больше потолка от лимита — предупредить.
//  2. Dump pre-flight: оценка по Size, без чтения содержимого.
//     Если переполнение — handleOverflow (fail/drop).
//  3. Чтение и (в clean) прогон через процессор.
//  4. Clean post-flight: точная оценка по Content.
//  5. Бюджетный фильтр (жадный, по Content).
//  6. Рендер в target.
//
// Куда идёт вывод:
//
//	--output               → файл
//	--clipboard            → только буфер, stdout молчит
//	--output + --clipboard → файл + буфер
//	без флагов             → stdout
func RunWith(opts Options, files []types.FileEntry) error {
	warnBudgetOverLimit(opts)

	// Dump pre-flight.
	if opts.Mode == ModeDump {
		contentTokens := 0
		for _, e := range files {
			contentTokens += tokens.EstimateSize(e.Size)
		}
		if r := Check(opts, files, CheckInput{
			ContentTokens: contentTokens,
			Exact:         true, // в dump Size/4 == len/4
		}); r != nil {
			if err := handleOverflow(&opts, r); err != nil {
				return err
			}
		}
	}

	// Process.
	procOpts := processor.Options{KeepDoc: opts.KeepDoc}
	processed := make([]types.ProcessedFile, 0, len(files))
	for _, e := range files {
		content, err := os.ReadFile(e.AbsPath)
		if err != nil {
			continue
		}
		if opts.Mode == ModeClean {
			p := processor.For(e.Path)
			out, err := p.Process(e.Path, content, procOpts)
			if err == nil {
				content = out
			}
		}
		processed = append(processed, types.ProcessedFile{
			Entry:   e,
			Content: content,
			Lines:   countLines(content),
		})
	}

	// Clean post-flight.
	if opts.Mode == ModeClean {
		contentTokens := 0
		for _, f := range processed {
			contentTokens += tokens.Estimate(f.Content)
		}
		if r := Check(opts, files, CheckInput{
			ContentTokens: contentTokens,
			Exact:         true,
		}); r != nil {
			if err := handleOverflow(&opts, r); err != nil {
				return err
			}
		}
	}

	// Бюджетный фильтр.
	processed, total, dropped := applyBudget(opts, processed)

	ctx := types.Context{
		Project: opts.Root,
		Root:    opts.Root,
		Files:   processed,
		Tokens:  total,
		Dropped: dropped,
	}

	var fileOut io.Writer
	if opts.Output != "" {
		f, err := os.Create(opts.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		fileOut = f
	}

	var buf bytes.Buffer
	var target io.Writer
	switch {
	case fileOut != nil && opts.Clipboard:
		target = io.MultiWriter(fileOut, &buf)
	case fileOut != nil:
		target = fileOut
	case opts.Clipboard:
		target = &buf
	default:
		target = os.Stdout
	}

	if err := render.Render(ctx, opts.Format, target); err != nil {
		return err
	}

	if opts.Clipboard {
		if err := clipboard.CopyToTTY(buf.Bytes()); err != nil {
			return fmt.Errorf("clipboard: %w", err)
		}
		writeSummary(opts.Summary, ctx, buf.Len())
	}
	return nil
}

// warnBudgetOverLimit печатает предупреждение, если --budget больше
// эффективного потолка от лимита (review §2.2, строка 6). Полезно
// в --on-overflow=drop: пользователь видит, что реально обрезал
// лимит, а не его бюджет.
func warnBudgetOverLimit(opts Options) {
	if opts.ContextLimit <= 0 || opts.Budget <= 0 {
		return
	}
	reserve := opts.Reserve
	if reserve <= 0 {
		reserve = tokens.DefaultReserve(opts.ContextLimit)
	}
	limitCeiling := opts.ContextLimit - reserve
	if opts.Budget > limitCeiling {
		fmt.Fprintf(opts.reportWriter(),
			"⚠ --budget %d больше потолка %d (limit %d − reserve %d); используется %d\n",
			opts.Budget, limitCeiling, opts.ContextLimit, reserve, limitCeiling)
	}
}

// applyBudget — жадный фильтр по содержимому. Если opts.Budget == 0
// — ничего не режет.
func applyBudget(opts Options, processed []types.ProcessedFile) ([]types.ProcessedFile, int, int) {
	total := 0
	dropped := 0
	if opts.Budget <= 0 {
		for _, f := range processed {
			total += tokens.Estimate(f.Content)
		}
		return processed, total, dropped
	}
	kept := processed[:0]
	for _, f := range processed {
		t := tokens.Estimate(f.Content)
		if total+t > opts.Budget {
			dropped++
			continue
		}
		total += t
		kept = append(kept, f)
	}
	return kept, total, dropped
}

func countLines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}
