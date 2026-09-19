package pipeline

import (
	"os"

	"github.com/it1ro/cake/internal/processor"
	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/internal/walker"
	"github.com/it1ro/cake/pkg/types"
)

type Mode int

const (
	ModeDump  Mode = iota // как есть
	ModeClean             // через процессор (strip comments и т.п.)
)

type Options struct {
	Root     string
	Includes []string
	Excludes []string
	MaxSize  int64
	Output   string
	Mode     Mode
	KeepDoc  bool
}

func Run(opts Options) error {
	entries, err := walker.Walk(walker.Options{
		Root:     opts.Root,
		Includes: opts.Includes,
		Excludes: opts.Excludes,
		MaxSize:  opts.MaxSize,
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

	ctx := types.Context{
		Project: opts.Root,
		Root:    opts.Root,
		Files:   processed,
	}

	out := os.Stdout
	if opts.Output != "" {
		f, err := os.Create(opts.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	return render.XML(ctx, out)
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
