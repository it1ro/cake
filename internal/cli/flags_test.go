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
// значения флагов в структуру и передавать их в applyFlags
// явно, а не через cmd.Flags().

// testFormat — приёмник для --format. Связывается с флагом в
// newTestCmd; applyFlags может переписать его из конфига.
var testFormat string

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

// newTestCmd строит свежую команду с теми же флагами, что
// навешаны в проде на rootCmd (persistent-набор) и на подкомандах
// (format/max-size/include/exclude/keep-doc/no-gitignore).
//
// opts передаётся, чтобы флаги --max-size, --include, --exclude
// биндились к тем же полям Options, что и в проде: applyFlags
// читает их как «значение флага» через Changed.
func newTestCmd(t *testing.T, opts *pipeline.Options, args ...string) *cobra.Command {
	t.Helper()
	testFormat = ""

	cmd := &cobra.Command{Use: "test"}
	pf := cmd.PersistentFlags()
	pf.StringVar(&flagContextLimit, "context-limit", "", "")
	pf.StringVar(&flagReserve, "reserve", "", "")
	pf.StringVar(&flagOnOverflow, "on-overflow", "", "")
	pf.StringVar(&flagReportFile, "report-file", "", "")
	pf.StringVar(&flagProfile, "profile", "", "")

	f := cmd.Flags()
	f.StringVar(&testFormat, "format", "xml", "")
	f.Int64Var(&opts.MaxSize, "max-size", 0, "")
	f.StringSliceVar(&opts.Includes, "include", nil, "")
	f.StringSliceVar(&opts.Excludes, "exclude", nil, "")
	f.Bool("keep-doc", false, "")
	f.Bool("no-gitignore", false, "")

	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v): %v", args, err)
	}
	return cmd
}

// apply — тонкая обёртка над applyFlags для тестов, которые
// не проверяют format. Использует testFormat как приёмник.
func apply(t *testing.T, cmd *cobra.Command, opts *pipeline.Options) error {
	t.Helper()
	return applyFlags(cmd, opts, &testFormat)
}

// ─── Базовые кейсы: ни конфига, ни флагов ───────────────────────────

func TestApplyFlags_NoConfigNoFlags(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts)
	if err := apply(t, cmd, &opts); err != nil {
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
	if opts.OnOverflow != pipeline.OverflowFail {
		t.Errorf("OnOverflow = %v, want OverflowFail (zero value)",
			opts.OnOverflow)
	}
}

// ─── Флаги без конфига ───────────────────────────────────────────────

func TestApplyFlags_CLIOnly(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts, "--context-limit", "200k", "--on-overflow", "drop")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 200_000 {
		t.Errorf("ContextLimit = %d, want 200000", opts.ContextLimit)
	}
	if opts.OnOverflow != pipeline.OverflowDrop {
		t.Errorf("OnOverflow = %v, want drop", opts.OnOverflow)
	}
}

func TestApplyFlags_ReservePercent(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts,
		"--context-limit", "200k",
		"--reserve", "10%",
	)
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 20_000 {
		t.Errorf("Reserve = %d, want 20000 (10%% от 200k)",
			opts.Reserve)
	}
}

func TestApplyFlags_ReserveAbsolute(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts,
		"--context-limit", "200k",
		"--reserve", "5k",
	)
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 5_000 {
		t.Errorf("Reserve = %d, want 5000", opts.Reserve)
	}
}

func TestApplyFlags_ReportFile(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts, "--report-file", "/tmp/report.json")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ReportFile != "/tmp/report.json" {
		t.Errorf("ReportFile = %q", opts.ReportFile)
	}
}

func TestApplyFlags_ReserveWithoutLimit(t *testing.T) {
	// --reserve 10% без --context-limit: ParseReserve получает
	// limit=0, процент от нуля = 0. Явно фиксируем, чтобы не
	// считать это багом позже.
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts, "--reserve", "10%")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 0 {
		t.Errorf("Reserve = %d, want 0 (10%% от нулевого лимита)",
			opts.Reserve)
	}
}

// ─── Конфиг без флагов ───────────────────────────────────────────────

func TestApplyFlags_ConfigDefault(t *testing.T) {
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
	cmd := newTestCmd(t, &opts)
	if err := apply(t, cmd, &opts); err != nil {
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

func TestApplyFlags_ConfigReservePercent(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
reserve       = "10%"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts)
	if err := apply(t, cmd, &opts); err != nil {
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
func TestApplyFlags_ZeroOverridesConfig(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts, "--context-limit", "0")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 0 {
		t.Errorf("ContextLimit = %d, want 0 (флаг перебивает конфиг)",
			opts.ContextLimit)
	}
}

func TestApplyFlags_CLIOverridesConfig(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
on-overflow   = "fail"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts,
		"--context-limit", "500k",
		"--on-overflow", "drop",
	)
	if err := apply(t, cmd, &opts); err != nil {
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
// должен считаться от лимита из конфига: applyFlags обрабатывает
// конфиг до разбора CLI-флагов, opts.ContextLimit уже 200k.
func TestApplyFlags_ConfigLimitPlusCLIReserve(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts, "--reserve", "10%")
	if err := apply(t, cmd, &opts); err != nil {
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

func TestApplyFlags_Profile(t *testing.T) {
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
	cmd := newTestCmd(t, &opts, "--profile", "cheap")
	if err := apply(t, cmd, &opts); err != nil {
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

func TestApplyFlags_ProfileWithCLIOverride(t *testing.T) {
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
	cmd := newTestCmd(t, &opts,
		"--profile", "cheap",
		"--context-limit", "64k",
	)
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ContextLimit != 64_000 {
		t.Errorf("ContextLimit = %d, want 64000 (CLI поверх профиля)",
			opts.ContextLimit)
	}
}

func TestApplyFlags_UnknownProfile(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts, "--profile", "nope")
	err := apply(t, cmd, &opts)
	if err == nil {
		t.Fatal("want error for unknown profile")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should mention profile name: %v", err)
	}
}

func TestApplyFlags_ProfileWithoutConfig(t *testing.T) {
	// Ни локального, ни глобального cake.toml — но --profile задан.
	// Это пользовательская ошибка: молча игнорировать нельзя.
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts, "--profile", "cheap")
	err := apply(t, cmd, &opts)
	if err == nil {
		t.Fatal("want error for --profile without cake.toml")
	}
	if !strings.Contains(err.Error(), "cheap") {
		t.Errorf("error should mention profile name: %v", err)
	}
}

// ─── Ошибки разбора ─────────────────────────────────────────────────

func TestApplyFlags_BadContextLimit(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts, "--context-limit", "abc")
	if err := apply(t, cmd, &opts); err == nil {
		t.Fatal("want error for bad --context-limit")
	}
}

func TestApplyFlags_BadReserve(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts,
		"--context-limit", "200k",
		"--reserve", "200%",
	)
	if err := apply(t, cmd, &opts); err == nil {
		t.Fatal("want error for reserve >= 100%")
	}
}

func TestApplyFlags_BadOnOverflow(t *testing.T) {
	isolateXDG(t)
	opts := pipeline.Options{Root: t.TempDir()}
	cmd := newTestCmd(t, &opts, "--on-overflow", "panic")
	if err := apply(t, cmd, &opts); err == nil {
		t.Fatal("want error for bad --on-overflow")
	}
}

// Конфиг с битым context-limit: ошибку должен вернуть applyFlags
// (config.Load числа не парсит — это ответственность tokens).
// Проверяем, что путь до cake.toml в ошибке сохранён.
func TestApplyFlags_BadConfigValue(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "abc"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts)
	err := apply(t, cmd, &opts)
	if err == nil {
		t.Fatal("want error for bad context-limit in config")
	}
	if !strings.Contains(err.Error(), "cake.toml") {
		t.Errorf("error should mention cake.toml: %v", err)
	}
}

func TestApplyFlags_BadConfigProfileValue(t *testing.T) {
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
	cmd := newTestCmd(t, &opts, "--profile", "cheap")
	if err := apply(t, cmd, &opts); err == nil {
		t.Fatal("want error for bad context-limit in profile")
	}
}

// ─── Порядок «конфиг → CLI» для резерва ─────────────────────────────

// Резерв в процентах считается от ФИНАЛЬНОГО лимита: CLI
// переопределяет context-limit, и reserve = "10%" должен дать
// 10% от нового значения. Регрессия на порядок «applyFlags
// съел reserve раньше CLI».
func TestApplyFlags_ReserveFromConfigRecomputedAgainstCLILimit(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
reserve       = "10%"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts, "--context-limit", "32k")
	if err := apply(t, cmd, &opts); err != nil {
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
func TestApplyFlags_ProfileLimitOverriddenReserveRecomputed(t *testing.T) {
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
	cmd := newTestCmd(t, &opts, "--profile", "cheap", "--context-limit", "64k")
	if err := apply(t, cmd, &opts); err != nil {
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
func TestApplyFlags_AbsoluteReserveKeptOnCLILimitOverride(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
context-limit = "200k"
reserve       = "5000"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts, "--context-limit", "32k")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Reserve != 5_000 {
		t.Errorf("Reserve = %d, want 5000", opts.Reserve)
	}
}

// ─── Остальные ключи cake.toml ──────────────────────────────────────

// format из конфига применяется, только если --format не задан.
func TestApplyFlags_FormatFromConfig(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
format = "markdown"
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts)
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if testFormat != "markdown" {
		t.Errorf("format = %q, want markdown (из конфига)", testFormat)
	}

	// А с --format plain — флаг побеждает.
	opts = pipeline.Options{Root: root}
	cmd = newTestCmd(t, &opts, "--format", "plain")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if testFormat != "plain" {
		t.Errorf("format = %q, want plain (флаг побеждает)", testFormat)
	}
}

// Списки include/exclude дополняются: конфиг + флаги.
func TestApplyFlags_ListsAppended(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
exclude = ["vendor/**", "testdata/**"]
include = ["**/*.go"]
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts,
		"--exclude", "**/*_test.go",
		"--include", "**/*.md",
	)
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}

	wantExcl := "vendor/**|testdata/**|**/*_test.go"
	if got := strings.Join(opts.Excludes, "|"); got != wantExcl {
		t.Errorf("Excludes = %q, want %q", got, wantExcl)
	}
	wantIncl := "**/*.go|**/*.md"
	if got := strings.Join(opts.Includes, "|"); got != wantIncl {
		t.Errorf("Includes = %q, want %q", got, wantIncl)
	}
}

// Явный сброс --exclude ” заменяет конфиг пустым списком.
// pflag на пустую CSV-строку даёт Changed=true и пустой срез —
// это и есть маркер сброса.
func TestApplyFlags_ExcludeReset(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
exclude = ["vendor/**"]
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts, "--exclude", "")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if len(opts.Excludes) != 0 {
		t.Errorf("Excludes = %v, want empty (reset)", opts.Excludes)
	}
}

// keep-doc, use-gitignore, max-size применяются из конфига, если
// флаг не задан.
func TestApplyFlags_KeepDocAndGitignore(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
keep-doc      = true
use-gitignore = false
max-size      = 4096
`)
	opts := pipeline.Options{
		Root:         root,
		UseGitignore: true, // как из флага «по умолчанию»
		MaxSize:      1 << 20,
	}
	cmd := newTestCmd(t, &opts)
	// Симулируем, что команда выставила UseGitignore из !noGitignore
	// (как в prod-RunE) — теперь конфиг должен перебить.
	opts.UseGitignore = true
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if !opts.KeepDoc {
		t.Error("KeepDoc should come from config")
	}
	if opts.UseGitignore {
		t.Error("UseGitignore = true, want false from config")
	}
	if opts.MaxSize != 4096 {
		t.Errorf("MaxSize = %d, want 4096", opts.MaxSize)
	}
}

// max-size из CLI побеждает конфиг.
func TestApplyFlags_MaxSizeFlagWins(t *testing.T) {
	isolateXDG(t)
	root := t.TempDir()
	writeCakeToml(t, root, `
version = 1

[default]
max-size = 4096
`)
	opts := pipeline.Options{Root: root}
	cmd := newTestCmd(t, &opts, "--max-size", "65536")
	if err := apply(t, cmd, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.MaxSize != 65536 {
		t.Errorf("MaxSize = %d, want 65536 (флаг побеждает)", opts.MaxSize)
	}
}
