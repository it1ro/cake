package cli

import (
	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
	"github.com/spf13/cobra"
)

var (
	cleanOpts        pipeline.Options
	cleanNoGitignore bool
	cleanKeepDoc     bool
	cleanFormat      string
	cleanClipboard   bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean [path]",
	Short: "Дамп Go-файлов без комментариев",
	Long: `Извлекает Go-код, удаляя обычные комментарии.

Doc-комментарии удаляются по умолчанию и сохраняются с --keep-doc.
Build-constraints (//go:build, // +build) и директивы компилятора
(//go:generate, //go:embed, //go:noinline, //line) сохраняются всегда.

С --clipboard вывод идёт в буфер, а не в stdout. Если задан
--output, файл тоже пишется. Отчёт о копировании — в stderr.

Учитывает .gitignore (корневой и вложенные) по умолчанию.
Отключается флагом --no-gitignore.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			cleanOpts.Root = args[0]
		}
		if cleanOpts.Root == "" {
			cleanOpts.Root = "."
		}
		cleanOpts.Clipboard = cleanClipboard
		cleanOpts.Mode = pipeline.ModeClean
		cleanOpts.KeepDoc = cleanKeepDoc
		cleanOpts.UseGitignore = !cleanNoGitignore

		if err := applyFlags(cmd, &cleanOpts, &cleanFormat); err != nil {
			return err
		}
		f, err := render.Parse(cleanFormat)
		if err != nil {
			return err
		}
		cleanOpts.Format = f
		return pipeline.Run(cleanOpts)
	},
}

func init() {
	cleanCmd.Flags().StringSliceVarP(&cleanOpts.Includes, "include", "i", nil,
		"glob-паттерны для включения (например, '**/*.go')")
	cleanCmd.Flags().StringSliceVarP(&cleanOpts.Excludes, "exclude", "e", nil,
		"glob-паттерны для исключения")
	cleanCmd.Flags().Int64Var(&cleanOpts.MaxSize, "max-size", 1<<20,
		"макс. размер файла в байтах (0 = без лимита)")
	cleanCmd.Flags().StringVarP(&cleanOpts.Output, "output", "o", "",
		"файл вывода (по умолчанию stdout)")
	cleanCmd.Flags().BoolVar(&cleanNoGitignore, "no-gitignore", false,
		"не учитывать .gitignore")
	cleanCmd.Flags().BoolVar(&cleanKeepDoc, "keep-doc", false,
		"сохранять doc-комментарии")
	cleanCmd.Flags().StringVar(&cleanFormat, "format", "xml",
		"формат вывода: xml | markdown | plain")
	cleanCmd.Flags().IntVar(&cleanOpts.Budget, "budget", 0,
		"лимит токенов; файлы сверх лимита пропускаются (0 = без лимита)")
	cleanCmd.Flags().BoolVar(&cleanClipboard, "clipboard", false,
		"скопировать вывод в буфер обмена (OSC 52); вместо stdout")
	rootCmd.AddCommand(cleanCmd)
}
