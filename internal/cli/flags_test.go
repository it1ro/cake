package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/it1ro/cake/internal/pipeline"
)

// Тесты в этом файле НЕ параллельны (t.Parallel не вызывается):
// persistent-флаги в registerLimitFlags привязаны к package-глобалам
// flagContextLimit и т.д., а newTestCmd перепривязывает их каждым
// вызовом. Гонка между параллельными тестами дала бы случайные
// значения. Если когда-нибудь понадобится параллельность — вынести
// значения флагов в структуру и передавать их в applyLimitFlags
// явно, а не через cmd.Flags().

// isolateXDG уводит XDG_CONFIG_HOME в tmp, чтобы глобальный
// cake.toml с реальной машины не подмешивался.
func isolateXDG(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func writeCakeToml(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(root, "cake.toml"), []byte(content), 0o644,
	); err != nil {
		t.Fatal(err)
	}
}

// newTestCmd строит свежую команду с теми же persistent-флагами,
// что registerLimitFlags вешает на rootCmd, и парсит args.
// После ParseFlags cmd.Flags() содержит и persistent-набор —
// Changed() работает как в проде.
func newTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	pf := cmd.PersistentFlags()
	pf.StringVar(&flagContextLimit, "context-limit", "", "")
	pf.StringVar(&flagReserve, "reserve", "", "")
	pf.StringVar(&flagOnOverflow, "on-overflow", "", "")
	pf.StringVar(&flagReportFile, "report-file", "", "")
	pf.StringVar(&flagProfile, "profile", "", "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v): %v", args, err)
	}
	return cmd
}

// ─── Базовые кейсы: ни конфига, ни флагов ───────────────────────────

func TestApplyLimitFlags_NoConfigNoFlags(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t)
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 0 {
		t.Errorf("ContextLimit = %d, want 0", opts.ContextLimit)
	}
	if opts.Reserve != 0 {
		t.Errorf("Reserve = %d, want 0 (auto вычислится позже)",
			opts.Reserve)
	}
	if opts.ReportFile != "" {
		t.Errorf("ReportFile = %q, want empty", opts.ReportFile)
	}
	// OnOverflow не трогаем: zero value = OverflowFail, это и есть дефолт.
	if opts.OnOverflow != pipeline.OverflowFail {
		t.Errorf("OnOverflow = %v, want OverflowFail (zero value)",
			opts.OnOverflow)
	}
}

// ─── Флаги без конфига ───────────────────────────────────────────────

func TestApplyLimitFlags_CLIOnly(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, "--context-limit", "200k", "--on-overflow", "drop")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 200_000 {
		t.Errorf("ContextLimit = %d, want 200000", opts.ContextLimit)
	}
	if opts.OnOverflow != pipeline.OverflowDrop {
		t.Errorf("OnOverflow = %v, want drop", opts.OnOverflow)
	}
}

func TestApplyLimitFlags_ReservePercent(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t,
		"--context-limit", "200k",
		"--reserve", "10%",
	)
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 20_000 {
		t.Errorf("Reserve = %d, want 20000 (10%% от 200k)",
			opts.Reserve)
	}
}

func TestApplyLimitFlags_ReserveAbsolute(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t,
		"--context-limit", "200k",
		"--reserve", "5k",
	)
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 5_000 {
		t.Errorf("Reserve = %d, want 5000", opts.Reserve)
	}
}

func TestApplyLimitFlags_ReportFile(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, "--report-file", "/tmp/report.json")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ReportFile != "/tmp/report.json" {
		t.Errorf("ReportFile = %q", opts.ReportFile)
	}
}

func TestApplyLimitFlags_ReserveWithoutLimit(t *testing.T) {
	// --reserve 10% без --context-limit: ParseReserve получает
	// limit=0, процент от нуля = 0. Явно фиксируем, чтобы не
	// считать это багом позже.
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, "--reserve", "10%")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 0 {
		t.Errorf("Reserve = %d, want 0 (10%% от нулевого лимита)",
			opts.Reserve)
	}
}

// ─── Конфиг без флагов ───────────────────────────────────────────────

func TestApplyLimitFlags_ConfigDefault(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
on-overflow   = "drop"
budget        = 150000
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t)
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 200_000 {
		t.Errorf("ContextLimit = %d, want 200000", opts.ContextLimit)
	}
	if opts.OnOverflow != pipeline.OverflowDrop {
		t.Errorf("OnOverflow = %v, want drop", opts.OnOverflow)
	}
	if opts.Budget != 150_000 {
		t.Errorf("Budget = %d, want 150000", opts.Budget)
	}
}

func TestApplyLimitFlags_ConfigReservePercent(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
reserve       = "10%"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t)
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 20_000 {
		t.Errorf("Reserve = %d, want 20000", opts.Reserve)
	}
}

// ─── Правило Changed: флаг > конфиг ─────────────────────────────────

// Ключевой контракт review §2.1: --context-limit 0 отключает
// лимит из cake.toml. Если когда-нибудь перейдём с Changed на
// «непустое значение», этот тест упадёт — и это правильно.
func TestApplyLimitFlags_ZeroOverridesConfig(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--context-limit", "0")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 0 {
		t.Errorf("ContextLimit = %d, want 0 (флаг перебивает конфиг)",
			opts.ContextLimit)
	}
}

func TestApplyLimitFlags_CLIOverridesConfig(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
on-overflow   = "fail"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t,
		"--context-limit", "500k",
		"--on-overflow", "drop",
	)
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 500_000 {
		t.Errorf("ContextLimit = %d, want 500000", opts.ContextLimit)
	}
	if opts.OnOverflow != pipeline.OverflowDrop {
		t.Errorf("OnOverflow = %v, want drop", opts.OnOverflow)
	}
}

// Config задаёт лимит, флаг — только резерв. Резерв (в процентах)
// должен считаться от лимита из конфига: applyProfile выполняется
// до разбора CLI-флагов, opts.ContextLimit уже 200k.
func TestApplyLimitFlags_ConfigLimitPlusCLIReserve(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--reserve", "10%")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 200_000 {
		t.Errorf("ContextLimit = %d, want 200000", opts.ContextLimit)
	}
	if opts.Reserve != 20_000 {
		t.Errorf("Reserve = %d, want 20000", opts.Reserve)
	}
}

// ─── Профили ────────────────────────────────────────────────────────

func TestApplyLimitFlags_Profile(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"

[profiles.cheap]
context-limit = "32k"
budget        = 30000
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--profile", "cheap")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 32_000 {
		t.Errorf("ContextLimit = %d, want 32000 (профиль)",
			opts.ContextLimit)
	}
	if opts.Budget != 30_000 {
		t.Errorf("Budget = %d, want 30000", opts.Budget)
	}
}

func TestApplyLimitFlags_ProfileWithCLIOverride(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"

[profiles.cheap]
context-limit = "32k"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t,
		"--profile", "cheap",
		"--context-limit", "64k",
	)
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 64_000 {
		t.Errorf("ContextLimit = %d, want 64000 (CLI поверх профиля)",
			opts.ContextLimit)
	}
}

func TestApplyLimitFlags_UnknownProfile(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--profile", "nope")
	err := applyLimitFlags(cmd, &opts)
	if err == nil {
		t.Fatal("want error for unknown profile")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should mention profile name: %v", err)
	}
}

func TestApplyLimitFlags_ProfileWithoutConfig(t *testing.T) {
	// Ни локального, ни глобального cake.toml — но --profile задан.
	// Это пользовательская ошибка: молча игнорировать нельзя.
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, "--profile", "cheap")
	err := applyLimitFlags(cmd, &opts)
	if err == nil {
		t.Fatal("want error for --profile without cake.toml")
	}
	if !strings.Contains(err.Error(), "cheap") {
		t.Errorf("error should mention profile name: %v", err)
	}
}

// ─── Ошибки разбора ─────────────────────────────────────────────────

func TestApplyLimitFlags_BadContextLimit(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, "--context-limit", "abc")
	if err := applyLimitFlags(cmd, &opts); err == nil {
		t.Fatal("want error for bad --context-limit")
	}
}

func TestApplyLimitFlags_BadReserve(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t,
		"--context-limit", "200k",
		"--reserve", "200%",
	)
	if err := applyLimitFlags(cmd, &opts); err == nil {
		t.Fatal("want error for reserve >= 100%")
	}
}

func TestApplyLimitFlags_BadOnOverflow(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, "--on-overflow", "panic")
	if err := applyLimitFlags(cmd, &opts); err == nil {
		t.Fatal("want error for bad --on-overflow")
	}
}

// Конфиг с битым context-limit: ошибку должен вернуть applyProfile
// (config.Load числа не парсит — это ответственность tokens).
// Проверяем, что путь до cake.toml в ошибке сохранён.
func TestApplyLimitFlags_BadConfigValue(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "abc"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t)
	err := applyLimitFlags(cmd, &opts)
	if err == nil {
		t.Fatal("want error for bad context-limit in config")
	}
	if !strings.Contains(err.Error(), "cake.toml") {
		t.Errorf("error should mention cake.toml: %v", err)
	}
}

func TestApplyLimitFlags_BadConfigProfileValue(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"

[profiles.cheap]
context-limit = "abc"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--profile", "cheap")
	if err := applyLimitFlags(cmd, &opts); err == nil {
		t.Fatal("want error for bad context-limit in profile")
	}
}

// ─── Известное ограничение ──────────────────────────────────────────

// Резерв в процентах считается от ФИНАЛЬНОГО лимита: CLI
// переопределяет context-limit, и reserve = "10%" должен дать
// 10% от нового значения. Регрессия на порядок «applyProfile
// съел reserve раньше CLI».
func TestApplyLimitFlags_ReserveFromConfigRecomputedAgainstCLILimit(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
reserve       = "10%"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--context-limit", "32k")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 32_000 {
		t.Errorf("ContextLimit = %d, want 32000", opts.ContextLimit)
	}
	if opts.Reserve != 3_200 {
		t.Errorf("Reserve = %d, want 3200 (10%% от финального 32k)",
			opts.Reserve)
	}
}

// Профиль задал контекст-лимит и reserve-процент. CLI перебивает
// лимит, но не резерв. Резерв должен пересчитаться от нового
// лимита, потому что это осмысленный «10% от того, что выбрал
// пользователь», а не от того, что было в профиле.
func TestApplyLimitFlags_ProfileLimitOverriddenReserveRecomputed(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"

[profiles.cheap]
context-limit = "32k"
reserve       = "10%"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--profile", "cheap", "--context-limit", "64k")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 64_000 {
		t.Errorf("ContextLimit = %d, want 64000", opts.ContextLimit)
	}
	if opts.Reserve != 6_400 {
		t.Errorf("Reserve = %d, want 6400 (10%% от 64k)", opts.Reserve)
	}
}

// Абсолютный reserve из конфига не пересчитывается, даже если
// CLI переопределил лимит. 5000 — это 5000, а не «сколько-то
// процентов».
func TestApplyLimitFlags_AbsoluteReserveKeptOnCLILimitOverride(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
reserve       = "5000"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, "--context-limit", "32k")
	if err := applyLimitFlags(cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 5_000 {
		t.Errorf("Reserve = %d, want 5000", opts.Reserve)
	}
}
