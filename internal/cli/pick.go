package cli

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/internal/tui"
)

var (
	pickOpts        pipeline.Options
	pickNoGitignore bool
	pickFormat      string
	pickClipboard   bool
)

var pickCmd = &cobra.Command{
	Use:   "pick [path]",
	Short: "Интерактивный выбор файлов",
	Long: `Открывает TUI со списком файлов проекта.
... (без изменений в тексте Long)
Учитывает .gitignore (корневой и вложенные) по умолчанию.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPick(cmd, args)
	},
}

// runPick — общая точка входа для `cake` (без аргументов) и
// `cake pick [path]`. Стартует TUI в tree-режиме.
//
// cmd нужен, чтобы прочитать persistent-флаги лимита (--profile,
// --context-limit, …). При вызове из `cake` без аргументов
// cobra передаёт rootCmd.
func runPick(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		pickOpts.Root = args[0]
	}
	if pickOpts.Root == "" {
		pickOpts.Root = "."
	}
	f, err := render.Parse(pickFormat)
	if err != nil {
		return err
	}
	pickOpts.Format = f
	pickOpts.Clipboard = pickClipboard
	pickOpts.Mode = pipeline.ModeDump
	pickOpts.UseGitignore = !pickNoGitignore
	if err := applyLimitFlags(cmd, &pickOpts); err != nil {
		return err
	}

	files, err := pipeline.Plan(pickOpts)
	if err != nil {
		return err
	}

	model := tui.New(files, pickOpts).WithTree()
	final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}

	fm, ok := final.(tui.Model)
	if !ok || !fm.Confirmed() {
		return nil // пользователь вышел без подтверждения
	}

	selected := fm.SelectedFiles()
	if len(selected) == 0 {
		return errors.New("ничего не выбрано")
	}

	return pipeline.RunWith(fm.Options(), selected)
}

func init() {
	pickCmd.Flags().StringSliceVarP(&pickOpts.Includes, "include", "i", nil,
		"glob-паттерны для включения")
	pickCmd.Flags().StringSliceVarP(&pickOpts.Excludes, "exclude", "e", nil,
		"glob-паттерны для исключения")
	pickCmd.Flags().Int64Var(&pickOpts.MaxSize, "max-size", 1<<20,
		"макс. размер файла в байтах (0 = без лимита)")
	pickCmd.Flags().StringVarP(&pickOpts.Output, "output", "o", "",
		"файл вывода (по умолчанию stdout)")
	pickCmd.Flags().BoolVar(&pickNoGitignore, "no-gitignore", false,
		"не учитывать .gitignore")
	pickCmd.Flags().StringVar(&pickFormat, "format", "xml",
		"стартовый формат вывода: xml | markdown | plain")
	pickCmd.Flags().BoolVar(&pickClipboard, "clipboard", false,
		"скопировать результат в буфер обмена (OSC 52); вместо stdout")
	pickCmd.Flags().BoolVar(&pickOpts.KeepDoc, "keep-doc", false,
		"сохранять doc-комментарии (только в режиме clean)")
	pickCmd.Flags().IntVar(&pickOpts.Budget, "budget", 0,
		"лимит токенов; файлы сверх лимита пропускаются (0 = без лимита)")
	rootCmd.AddCommand(pickCmd)
}
