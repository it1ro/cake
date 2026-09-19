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
	ModeDump  Mode = iota // как есть
	ModeClean             // через процессор (strip comments и т.п.)
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

	// Budget — лимит токенов. 0 = без лимита.
	// Файлы, не влезающие в лимит, пропускаются (жадно);
	// порядок оставшихся сохраняется.
	Budget int

	// Clipboard — дополнительно отправить вывод в буфер
	// обмена через OSC 52.
	Clipboard bool
}

func Run(opts Options) error {
	entries, err := walker.Walk(walker.Options{
		Root:         opts.Root,
		Includes:     opts.Includes,
		Excludes:     opts.Excludes,
		MaxSize:      opts.MaxSize,
		UseGitignore: opts.UseGitignore,
	})
	if err != nil {
		return err
	}

	procOpts := processor.Options{KeepDoc: opts.KeepDoc}
	processed := make([]types.ProcessedFile, 0, len(entries))
	for _, e := range entries {
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

	// Токены + бюджет.
	// Жадный фильтр: если файл не влезает — пропускаем его и
	// пробуем следующий (в отсортированном списке мелкий файл
	// может пройти после крупного). Порядок сохраняется.
	total := 0
	dropped := 0
	if opts.Budget > 0 {
		kept := processed[:0] // reuse backing array
		for _, f := range processed {
			t := tokens.Estimate(f.Content)
			if total+t > opts.Budget {
				dropped++
				continue
			}
			total += t
			kept = append(kept, f)
		}
		processed = kept
	} else {
		for _, f := range processed {
			total += tokens.Estimate(f.Content)
		}
	}

	ctx := types.Context{
		Project: opts.Root,
		Root:    opts.Root,
		Files:   processed,
		Tokens:  total,
		Dropped: dropped,
	}

	out := io.Writer(os.Stdout)
	if opts.Output != "" {
		f, err := os.Create(opts.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	// Если нужен clipboard, tee'им вывод в буфер — OSC 52 требует
	// base64 от полного содержимого. Дублирование памяти тут
	// осознанное: включается только по флагу.
	var buf bytes.Buffer
	target := out
	if opts.Clipboard {
		target = io.MultiWriter(out, &buf)
	}

	if err := render.Render(ctx, opts.Format, target); err != nil {
		return err
	}

	if opts.Clipboard {
		if err := clipboard.CopyToTTY(buf.Bytes()); err != nil {
			return fmt.Errorf("clipboard: %w", err)
		}
	}
	return nil
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
