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

// applyFlags накатывает cake.toml (если есть), затем переопределяет
// значения из явно установленных CLI-флагов (Changed).
//
// Приоритет: флаг > профиль > [default] > глобальный > встроенный
// дефолт. Списки (include/exclude) дополняются: сначала конфиг,
// затем флаги. Явный сброс `--exclude ”` заменяет конфиг пустым
// списком.
//
// format — указатель на строковую переменную команды
// (dumpFormat / cleanFormat / pickFormat). Формат — свойство
// команды, не pipeline: applyFlags кладёт в него значение из
// конфига, только если флаг --format не был задан явно.
//
// Резерв в процентах разбирается один раз, в самом конце, против
// финального ContextLimit (после всех Changed-переопределений).
// Иначе при `cake.toml: reserve = "10%"` и `--context-limit 32k`
// резерв считался бы от конфиг-лимита, а не от 32k.
//
// --profile без cake.toml — ошибка: пользователь явно указал
// профиль, которого нет.
func applyFlags(cmd *cobra.Command, opts *pipeline.Options, format *string) error {
	loaded, err := config.Load(opts.Root, flagProfile)
	if err != nil {
		return err
	}
	if loaded == nil && flagProfile != "" {
		return fmt.Errorf("--profile %q: cake.toml not found", flagProfile)
	}

	f := cmd.Flags()

	// Снимок списков до слияния с конфигом: pflag уже положил
	// сюда значения из --include/--exclude, если они были заданы;
	// иначе — nil (дефолт StringSliceVarP). После mergeList
	// opts.Includes/Excludes будут перезаписаны.
	flagIncludes := opts.Includes
	flagExcludes := opts.Excludes
	hasFlagIncludes := f.Changed("include")
	hasFlagExcludes := f.Changed("exclude")

	var cfgReserve string
	if loaded != nil {
		p := loaded.Resolved

		if p.ContextLimit != "" {
			n, err := tokens.ParseCount(p.ContextLimit)
			if err != nil {
				return fmt.Errorf("cake.toml: context-limit: %w", err)
			}
			opts.ContextLimit = n
		}
		cfgReserve = p.Reserve

		if p.OnOverflow != "" {
			switch p.OnOverflow {
			case "fail":
				opts.OnOverflow = pipeline.OverflowFail
			case "drop":
				opts.OnOverflow = pipeline.OverflowDrop
			default:
				return fmt.Errorf(
					"cake.toml: on-overflow: want fail|drop, got %q",
					p.OnOverflow)
			}
		}
		if p.Budget != nil {
			opts.Budget = *p.Budget
		}
		if p.MaxSize != nil && !f.Changed("max-size") {
			opts.MaxSize = *p.MaxSize
		}
		if p.KeepDoc != nil && !f.Changed("keep-doc") {
			opts.KeepDoc = *p.KeepDoc
		}
		if p.UseGitignore != nil && !f.Changed("no-gitignore") {
			opts.UseGitignore = *p.UseGitignore
		}
		if p.Format != "" && !f.Changed("format") {
			*format = p.Format
		}

		opts.Includes = mergeList(p.Include, flagIncludes, hasFlagIncludes)
		opts.Excludes = mergeList(p.Exclude, flagExcludes, hasFlagExcludes)
	}

	if f.Changed("context-limit") {
		n, err := tokens.ParseCount(flagContextLimit)
		if err != nil {
			return err
		}
		opts.ContextLimit = n
	}

	// Резерв: CLI > конфиг. Оба разбираются против финального
	// ContextLimit. Пустая cfgReserve + не заданный флаг →
	// opts.Reserve == 0, дальше сработает DefaultReserve в
	// effectiveCeiling.
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

// mergeList склеивает список из конфига и список из флагов
// (конфиг впереди, флаги дополняют). Исключение — явный сброс:
// `--exclude ”` в pflag даёт Changed=true и пустой срез
// (readAsCSV("") → []string{}), трактуем это как «очистить
// список конфига».
func mergeList(cfg, flags []string, flagChanged bool) []string {
	if flagChanged && len(flags) == 0 {
		return nil
	}
	if len(cfg) == 0 && len(flags) == 0 {
		return nil
	}
	out := make([]string, 0, len(cfg)+len(flags))
	out = append(out, cfg...)
	out = append(out, flags...)
	return out
}
