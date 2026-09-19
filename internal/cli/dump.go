package cli

import (
	"github.com/it1ro/cake/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	dumpOpts        pipeline.Options
	dumpNoGitignore bool
)

var dumpCmd = &cobra.Command{
	Use:   "dump [path]",
	Short: "Полный тегированный дамп проекта",
	Long: `Собирает все файлы проекта в XML-подобный формат с деревом
в начале и содержимым каждого файла в CDATA-блоке.

Учитывает .gitignore (корневой и вложенные) по умолчанию.
Отключается флагом --no-gitignore.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			dumpOpts.Root = args[0]
		}
		if dumpOpts.Root == "" {
			dumpOpts.Root = "."
		}
		dumpOpts.Mode = pipeline.ModeDump
		dumpOpts.UseGitignore = !dumpNoGitignore
		return pipeline.Run(dumpOpts)
	},
}

func init() {
	dumpCmd.Flags().StringSliceVarP(&dumpOpts.Includes, "include", "i", nil,
		"glob-паттерны для включения (например, '**/*.go')")
	dumpCmd.Flags().StringSliceVarP(&dumpOpts.Excludes, "exclude", "e", nil,
		"glob-паттерны для исключения")
	dumpCmd.Flags().Int64Var(&dumpOpts.MaxSize, "max-size", 1<<20,
		"макс. размер файла в байтах (0 = без лимита)")
	dumpCmd.Flags().StringVarP(&dumpOpts.Output, "output", "o", "",
		"файл вывода (по умолчанию stdout)")
	dumpCmd.Flags().BoolVar(&dumpNoGitignore, "no-gitignore", false,
		"не учитывать .gitignore")
	rootCmd.AddCommand(dumpCmd)
}
