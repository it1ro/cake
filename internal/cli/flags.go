package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/it1ro/cake/internal/config"
	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/tokens"
)

var (
	flagContextLimit string
	flagReserve      string
	flagOnOverflow   string
	flagReportFile   string
	flagProfile      string
)

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
	pf.StringVar(&flagProfile, "profile", "",
		"профиль из cake.toml (по умолчанию [default])")
}

// applyLimitFlags накатывает cake.toml (если есть), затем
// переопределяет значения из явно установленных CLI-флагов
// (Changed). Правило: флаг > профиль > [default] > глобальный
// > встроенный дефолт.
//
// Резерв в процентах разбирается один раз — против финального
// ContextLimit, после CLI-переопределений. Иначе при
// `cake.toml: reserve = "10%"` и `--context-limit 32k` резерв
// считался бы от конфиг-лимита, а не от 32k.
//
// --profile без cake.toml — ошибка: пользователь явно указал
// профиль, которого нет.
func applyLimitFlags(cmd *cobra.Command, opts *pipeline.Options) error {
	loaded, err := config.Load(opts.Root, flagProfile)
	if err != nil {
		return err
	}
	if loaded == nil && flagProfile != "" {
		return fmt.Errorf("--profile %q: cake.toml not found", flagProfile)
	}

	// Скаляры — сразу. Резерв — отдельно, после CLI.
	var cfgReserve string
	if loaded != nil {
		if err := applyProfile(opts, loaded.Resolved); err != nil {
			return err
		}
		cfgReserve = loaded.Resolved.Reserve
	}

	f := cmd.Flags()

	if f.Changed("context-limit") {
		n, err := tokens.ParseCount(flagContextLimit)
		if err != nil {
			return err
		}
		opts.ContextLimit = n
	}

	// Резерв: CLI > конфиг. Оба разбираются против уже
	// финального ContextLimit. Пустая cfgReserve + не заданный
	// флаг → opts.Reserve == 0, дальше сработает DefaultReserve
	// в effectiveCeiling.
	switch {
	case f.Changed("reserve"):
		n, err := tokens.ParseReserve(flagReserve, opts.ContextLimit)
		if err != nil {
			return err
		}
		opts.Reserve = n
	case cfgReserve != "":
		n, err := tokens.ParseReserve(cfgReserve, opts.ContextLimit)
		if err != nil {
			return fmt.Errorf("cake.toml: reserve: %w", err)
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

// applyProfile кладёт в Options всё, кроме резерва: резерв
// разбирается отдельно, против финального ContextLimit.
func applyProfile(opts *pipeline.Options, p config.Profile) error {
	if p.ContextLimit != "" {
		n, err := tokens.ParseCount(p.ContextLimit)
		if err != nil {
			return fmt.Errorf("cake.toml: context-limit: %w", err)
		}
		opts.ContextLimit = n
	}
	if p.OnOverflow != "" {
		switch p.OnOverflow {
		case "fail":
			opts.OnOverflow = pipeline.OverflowFail
		case "drop":
			opts.OnOverflow = pipeline.OverflowDrop
		default:
			return fmt.Errorf(
				"cake.toml: on-overflow: want fail|drop, got %q", p.OnOverflow)
		}
	}
	if p.Budget != nil {
		opts.Budget = *p.Budget
	}
	return nil
}
