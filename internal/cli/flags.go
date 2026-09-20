package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/tokens"
)

var (
	flagContextLimit string
	flagReserve      string
	flagOnOverflow   string
	flagReportFile   string
)

// registerLimitFlags вешает persistent-флаги лимита на rootCmd.
// Так они доступны во всех подкомандах и в `cake <path>`.
func registerLimitFlags() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagContextLimit, "context-limit", "",
		"лимит контекста модели: N, 200k, 1m (0 = отключить)")
	pf.StringVar(&flagReserve, "reserve", "",
		"резерв под system prompt и ответ: N или N%")
	pf.StringVar(&flagOnOverflow, "on-overflow", "",
		"реакция на переполнение: fail | drop (по умолчанию fail)")
	pf.StringVar(&flagReportFile, "report-file", "",
		"путь к JSON-отчёту о переполнении")
}

// applyLimitFlags разбирает persistent-флаги и заполняет opts.
// Правило: флаг побеждает, только если Changed (review §D9 — на
// будущее для слияния с cake.toml). Иначе — не трогаем.
func applyLimitFlags(cmd *cobra.Command, opts *pipeline.Options) error {
	f := cmd.Flags()

	if f.Changed("context-limit") {
		n, err := tokens.ParseCount(flagContextLimit)
		if err != nil {
			return err
		}
		opts.ContextLimit = n
	}
	if f.Changed("reserve") {
		n, err := tokens.ParseReserve(flagReserve, opts.ContextLimit)
		if err != nil {
			return err
		}
		opts.Reserve = n
	}
	if f.Changed("on-overflow") {
		switch flagOnOverflow {
		case "fail":
			opts.OnOverflow = pipeline.OverflowFail
		case "drop":
			opts.OnOverflow = pipeline.OverflowDrop
		default:
			return fmt.Errorf(
				"--on-overflow: want fail|drop, got %q", flagOnOverflow)
		}
	}
	if f.Changed("report-file") {
		opts.ReportFile = flagReportFile
	}
	return nil
}
