package cli

import (
	"github.com/it1ro/cake/internal/pipeline"
	"github.com/spf13/cobra"
)

var dumpOpts pipeline.Options

var dumpCmd = &cobra.Command{
	Use:   "dump [path]",
	Short: "Полный тегированный дамп проекта",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			dumpOpts.Root = args[0]
		}
		if dumpOpts.Root == "" {
			dumpOpts.Root = "."
		}
		dumpOpts.Mode = pipeline.ModeDump
		return pipeline.Run(dumpOpts)
	},
}

func init() {
	dumpCmd.Flags().StringSliceVarP(&dumpOpts.Includes, "include", "i", nil, "glob-паттерны для включения")
	dumpCmd.Flags().StringSliceVarP(&dumpOpts.Excludes, "exclude", "e", nil, "glob-паттерны для исключения")
	dumpCmd.Flags().Int64Var(&dumpOpts.MaxSize, "max-size", 1<<20, "макс. размер файла в байтах")
	dumpCmd.Flags().StringVarP(&dumpOpts.Output, "output", "o", "", "файл вывода (по умолчанию stdout)")
	rootCmd.AddCommand(dumpCmd)
}
