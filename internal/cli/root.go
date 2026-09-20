package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
)

var rootCmd = &cobra.Command{
	Use:   "cake [path]",
	Short: "Извлекает контекст из кодовой базы для LLM",
	Long: `cake — CLI для извлечения контекста из кода в форматы,
удобные для LLM.

Без аргументов открывает интерактивный выбор файлов (pick)
сразу в древовидном режиме.

С путём — быстрое копирование всего проекта (кроме .gitignore)
в буфер обмена. Содержимое в stdout не пишется, краткий отчёт —
в stderr. Эквивалент "cake dump <path> --clipboard".

Команды:
  dump  [path]   полный тегированный дамп
  clean [path]   Go-код без комментариев
  pick  [path]   интерактивный выбор файлов

Коды выхода:
  0  успех
  1  ошибка (не смогли прочитать файл и т.п.)
  3  переполнение контекста (при --on-overflow=fail)`,
	SilenceUsage:  true,
	SilenceErrors: true, // печать ошибок — только в main.go
	Args:          cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runPick(cmd, nil)
		}
		return runQuickDump(cmd, args[0])
	},
}

// runQuickDump — неинтерактивный сценарий "cake <path>":
// дамп всего проекта (с учётом .gitignore) в буфер обмена.
//
// Format получается через applyFlags → render.Parse, чтобы
// `format = "markdown"` в cake.toml работал и для этой точки
// входа, а не только для подкоманд.
func runQuickDump(cmd *cobra.Command, root string) error {
	opts := pipeline.Options{
		Root:         root,
		Mode:         pipeline.ModeDump,
		UseGitignore: true,
		MaxSize:      1 << 20,
		Clipboard:    true,
	}
	format := "xml"
	if err := applyFlags(cmd, &opts, &format); err != nil {
		return err
	}
	f, err := render.Parse(format)
	if err != nil {
		return err
	}
	opts.Format = f
	return pipeline.Run(opts)
}

// Execute — точка входа CLI. Вызывается из cmd/cake/main.go.
func Execute() error { return rootCmd.Execute() }

func SetVersion(v, c, d string) {
	rootCmd.Version = fmt.Sprintf("%s (%s, %s)", v, c, d)
}

func init() {
	registerLimitFlags()
}
