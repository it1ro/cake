package cli

import (
	"github.com/it1ro/cake/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	cleanOpts    pipeline.Options
	cleanKeepDoc bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean [path]",
	Short: "Дамп Go-файлов без комментариев",
	Long: `Извлекает Go-код, удаляя обычные комментарии.
Doc-комментарии удаляются по умолчанию, сохраняются с --keep-doc.
Build-constraints и //go:* директивы сохраняются всегда.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			cleanOpts.Root = args[0]
		}
		if cleanOpts.Root == "" {
			cleanOpts.Root = "."
		}
		cleanOpts.Mode = pipeline.ModeClean
		cleanOpts.KeepDoc = cleanKeepDoc
		return pipeline.Run(cleanOpts)
	},
}

func init() {
	cleanCmd.Flags().StringSliceVarP(&cleanOpts.Includes, "include", "i", nil, "glob-паттерны для включения")
	cleanCmd.Flags().StringSliceVarP(&cleanOpts.Excludes, "exclude", "e", nil, "glob-паттерны для исключения")
	cleanCmd.Flags().Int64Var(&cleanOpts.MaxSize, "max-size", 1<<20, "макс. размер файла в байтах")
	cleanCmd.Flags().StringVarP(&cleanOpts.Output, "output", "o", "", "файл вывода (по умолчанию stdout)")
	cleanCmd.Flags().BoolVar(&cleanKeepDoc, "keep-doc", false, "сохранять doc-комментарии")
	rootCmd.AddCommand(cleanCmd)
}
