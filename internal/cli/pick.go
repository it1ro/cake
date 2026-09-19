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

Навигация:
  ↑/↓ или j/k      перемещение
  space            выбрать/снять файл
  a / A            выбрать / снять всё видимое
  d                toggle всей директории текущего файла
  /                fuzzy-фильтр по пути (esc — сбросить)
  tab              переключить режим dump ↔ clean
  f                переключить формат xml → markdown → plain
  ⏎                экспорт (pipeline.RunWith)
  q / ctrl+c       выход без вывода

Учитывает .gitignore (корневой и вложенные) по умолчанию.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		files, err := pipeline.Plan(pickOpts)
		if err != nil {
			return err
		}

		model := tui.New(files, pickOpts)
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
	},
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
		"скопировать результат в буфер обмена (OSC 52)")
	pickCmd.Flags().BoolVar(&pickOpts.KeepDoc, "keep-doc", false,
		"сохранять doc-комментарии (только в режиме clean)")
	pickCmd.Flags().IntVar(&pickOpts.Budget, "budget", 0,
		"лимит токенов; файлы сверх лимита пропускаются (0 = без лимита)")
	rootCmd.AddCommand(pickCmd)
}
