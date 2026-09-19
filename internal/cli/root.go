package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cake",
	Short: "Извлекает контекст из кодовой базы для LLM",
	Long: `cake — CLI для извлечения контекста из кода в форматы,
удобные для LLM: dump (всё с тегами), clean (Go без комментариев),
pick (интерактивный выбор файлов).`,
	SilenceUsage: true,
}

// Execute — точка входа CLI. Вызывается из cmd/cake/main.go.
func Execute() error { return rootCmd.Execute() }

func SetVersion(v, c, d string) {
	rootCmd.Version = fmt.Sprintf("%s (%s, %s)", v, c, d)
}
