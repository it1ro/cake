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
	Budget       int
	Clipboard    bool
}

// Plan — обход ФС и применение фильтров (gitignore, include/exclude,
// max-size, отсечение бинарников). Возвращает отсортированный список.
//
// Отдельно от Run, чтобы TUI мог показать файлы до фактического
// рендера и дать пользователю отредактировать выбор.
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
// files — обычно результат Plan; TUI может передать отредактированный
// пользователем подсписок.
func RunWith(opts Options, files []types.FileEntry) error {
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

	total := 0
	dropped := 0
	if opts.Budget > 0 {
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
