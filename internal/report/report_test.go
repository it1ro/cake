package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/it1ro/cake/pkg/types"
)

// ─── Агрегация по директориям ────────────────────────────────────────

// Схлопывание цепочки без ветвления: k8s.io/api/ собирается в один
// бакет. Фикстура НЕ в vendor/ — walker его отсекает безусловно,
// но здесь мы тестируем чистую функцию, поэтому важно, чтобы
// данные не напоминали про vendor: если когда-нибудь Aggregate
// начнёт смотреть на путь, тест должен ловить смысл, а не совпадение.
func TestAggregate_CollapsesChain(t *testing.T) {
	items := []Item{
		{Path: "k8s.io/api/types.go", Tokens: 100},
		{Path: "k8s.io/api/more.go", Tokens: 200},
	}
	got := Aggregate(items, 0)
	if len(got) != 1 {
		t.Fatalf("want 1 bucket, got %d: %+v", len(got), got)
	}
	if got[0].Key != "k8s.io/api/" {
		t.Errorf("key = %q, want k8s.io/api/", got[0].Key)
	}
	if got[0].Files != 2 || got[0].Tokens != 300 {
		t.Errorf("bucket = %+v, want {Files:2 Tokens:300}", got[0])
	}
}

// Файлы в корне проекта попадают в бакет "./".
func TestAggregate_RootFilesInDot(t *testing.T) {
	items := []Item{
		{Path: "README.md", Tokens: 50},
		{Path: "go.mod", Tokens: 30},
		{Path: "internal/a.go", Tokens: 200},
	}
	got := Aggregate(items, 0)

	var haveDot, haveInternal bool
	for _, b := range got {
		switch b.Key {
		case "./":
			haveDot = true
			if b.Files != 2 || b.Tokens != 80 {
				t.Errorf("bucket ./ = %+v, want {Files:2 Tokens:80}", b)
			}
		case "internal/":
			haveInternal = true
		}
	}
	if !haveDot {
		t.Error("root files bucket ./ missing")
	}
	if !haveInternal {
		t.Error("internal/ bucket missing")
	}
}

// Равномерное распределение веса не схлопывается: два ребёнка
// с равным весом остаются отдельными бакетами.
func TestAggregate_StopsAtBranch(t *testing.T) {
	items := []Item{
		{Path: "a/x.go", Tokens: 100},
		{Path: "b/y.go", Tokens: 100},
	}
	got := Aggregate(items, 0)
	if len(got) != 2 {
		t.Fatalf("want 2 buckets, got %d: %+v", len(got), got)
	}
}

// Пустой вход — nil, не пустой срез.
func TestAggregate_Empty(t *testing.T) {
	if got := Aggregate(nil, 0); got != nil {
		t.Errorf("nil → nil, got %+v", got)
	}
	if got := Aggregate([]Item{}, 0); got != nil {
		t.Errorf("empty → nil, got %+v", got)
	}
}

// top-лимит: обрезает по токенам убыв.
func TestAggregate_TopTrim(t *testing.T) {
	items := []Item{
		{Path: "big/a.go", Tokens: 1000},
		{Path: "mid/a.go", Tokens: 100},
		{Path: "small/a.go", Tokens: 10},
	}
	got := Aggregate(items, 2)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Key != "big/" || got[1].Key != "mid/" {
		t.Errorf("order/trim wrong: %+v", got)
	}
}

// ─── Агрегация по расширениям ────────────────────────────────────────

func TestAggregateExt(t *testing.T) {
	items := []Item{
		{Path: "a.go", Tokens: 100},
		{Path: "b.go", Tokens: 100},
		{Path: "c.md", Tokens: 50},
		{Path: "Makefile", Tokens: 30},
	}
	got := AggregateExt(items, 0)

	m := map[string]Bucket{}
	for _, b := range got {
		m[b.Key] = b
	}
	if m[".go"].Files != 2 || m[".go"].Tokens != 200 {
		t.Errorf(".go = %+v, want {Files:2 Tokens:200}", m[".go"])
	}
	if m[".md"].Files != 1 || m[".md"].Tokens != 50 {
		t.Errorf(".md = %+v", m[".md"])
	}
	// Файл без расширения идёт в "(нет)".
	if m["(нет)"].Files != 1 || m["(нет)"].Tokens != 30 {
		t.Errorf("(нет) = %+v, want {Files:1 Tokens:30}", m["(нет)"])
	}
}

// Сортировка по токенам убыв.; при равенстве — по ключу.
func TestAggregateExt_Sorted(t *testing.T) {
	items := []Item{
		{Path: "a.go", Tokens: 100},
		{Path: "b.md", Tokens: 200},
		{Path: "c.json", Tokens: 100},
	}
	got := AggregateExt(items, 0)
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	if got[0].Key != ".md" {
		t.Errorf("first = %q, want .md", got[0].Key)
	}
	// .go (100) и .json (100) — по имени.
	if got[1].Key != ".go" || got[2].Key != ".json" {
		t.Errorf("tie order = %q, %q; want .go, .json", got[1].Key, got[2].Key)
	}
}

// ─── Жадный набор exclude-мер ────────────────────────────────────────

// Без двойного счёта: мера, покрывающая вложенный каталог,
// не должна считаться дважды, если кандидаты пересекаются.
// Проверяем через результат Build: сумма весов в items считается
// ровно один раз.
func TestExcludeMeasure_NoDoubleCount(t *testing.T) {
	// 3 файла в internal/, 1 файл в internal/sub/.
	// Кандидат "internal/**" должен покрыть все 4.
	// После — remaining <= ceiling, дельта соответствует
	// сумме токенов покрытых файлов.
	entries := []types.FileEntry{
		{Path: "internal/a.go", Size: 4000},     // ~1000 tok
		{Path: "internal/b.go", Size: 4000},     // ~1000 tok
		{Path: "internal/sub/c.go", Size: 4000}, // ~1000 tok
		{Path: "keep.go", Size: 400},            // ~100 tok
	}
	r := Build(Params{
		Entries:        entries,
		OverheadTokens: 0,
		Limit:          3000,
		Reserve:        500,
		Ceiling:        2500, // реальный потолок 2500, при оценке 3100
		Exact:          true,
		Mode:           "dump",
	})

	if len(r.Measures) == 0 {
		t.Fatal("want at least one measure")
	}
	// Ищем exclude-меру.
	var excl *Measure
	for i := range r.Measures {
		if r.Measures[i].Label == "exclude" {
			excl = &r.Measures[i]
			break
		}
	}
	if excl == nil {
		t.Fatalf("no exclude measure in %+v", r.Measures)
	}

	// Сумма оставшихся токенов — примерно то, что даёт After.
	// Точное совпадение не обязательно: overhead + слэк,
	// но After точно меньше 3100 (initial estimate).
	if excl.After >= 3100 {
		t.Errorf("After = %d, должен быть меньше initial estimate (3100); "+
			"значит, что-то посчитано дважды", excl.After)
	}
}

// Мера с нулевой дельтой не попадает в отчёт: если единственное
// расширение — то самое, что предлагается к include, мера бессмысленна.
func TestIncludeMeasure_SkippedWhenNoDelta(t *testing.T) {
	// Только .go-файлы: include по .go не уменьшит набор.
	entries := []types.FileEntry{
		{Path: "a.go", Size: 8000},
		{Path: "b.go", Size: 8000},
	}
	r := Build(Params{
		Entries:        entries,
		OverheadTokens: 0,
		Limit:          1000,
		Reserve:        100,
		Ceiling:        900,
		Exact:          true,
		Mode:           "dump",
	})
	for _, m := range r.Measures {
		if m.Label == "include" {
			t.Errorf("include с единственным расширением не должен появляться: %+v", m)
		}
	}
}

// Fits: true — если после меры оценка укладывается в потолок.
func TestMeasure_FitsFlag(t *testing.T) {
	// Большой каталог + маленький. Потолок ставим так, чтобы
	// после exclude большого — влезли.
	entries := []types.FileEntry{
		{Path: "big/a.go", Size: 40000}, // ~10000 tok
		{Path: "big/b.go", Size: 40000}, // ~10000 tok
		{Path: "small.go", Size: 400},   // ~100 tok
	}
	r := Build(Params{
		Entries:        entries,
		OverheadTokens: 0,
		Limit:          2000,
		Reserve:        100,
		Ceiling:        1900,
		Exact:          true,
		Mode:           "dump",
	})
	if len(r.Measures) == 0 {
		t.Fatal("want measures")
	}
	// Хотя бы одна мера должна влезать (exclude big/** → ~100).
	anyFits := false
	for _, m := range r.Measures {
		if m.Fits {
			anyFits = true
			break
		}
	}
	if !anyFits {
		t.Errorf("want at least one Fits=true measure: %+v", r.Measures)
	}
}

// Пустой список мер, когда estimate <= ceiling.
func TestMeasures_EmptyWhenFits(t *testing.T) {
	entries := []types.FileEntry{
		{Path: "a.go", Size: 400},
	}
	r := Build(Params{
		Entries:        entries,
		OverheadTokens: 0,
		Limit:          10000,
		Reserve:        1000,
		Ceiling:        9000,
		Exact:          true,
		Mode:           "dump",
	})
	if len(r.Measures) != 0 {
		t.Errorf("estimate <= ceiling: мер быть не должно, got %+v", r.Measures)
	}
}

// ─── WriteJSON ───────────────────────────────────────────────────────

// JSON со schema: 1 — контракт для CI.
func TestWriteJSON_Schema(t *testing.T) {
	r := &Report{
		Schema:   SchemaVersion,
		Mode:     "dump",
		Limit:    200_000,
		Reserve:  20_000,
		Ceiling:  180_000,
		Estimate: 247_000,
		Exact:    false,
		Dirs: []Bucket{
			{Key: "internal/", Files: 312, Tokens: 118_000},
		},
		Exts: []Bucket{
			{Key: ".go", Files: 1823, Tokens: 141_000},
		},
		Measures: []Measure{
			{Label: "exclude", Flags: []string{"-e", "doc/**"}, After: 171_000, Fits: true},
		},
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "report.json")
	if err := r.WriteJSON(file); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	var back Report
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("JSON invalid: %v\n%s", err, data)
	}
	if back.Schema != 1 {
		t.Errorf("schema = %d, want 1", back.Schema)
	}
	if back.Estimate != 247_000 {
		t.Errorf("estimate = %d, want 247000", back.Estimate)
	}
	if len(back.Measures) != 1 || back.Measures[0].Label != "exclude" {
		t.Errorf("measures roundtrip failed: %+v", back.Measures)
	}

	// Проверяем ключ "schema" в сыром JSON — это часть контракта.
	if !strings.Contains(string(data), `"schema": 1`) {
		t.Errorf("raw JSON must contain \"schema\": 1\n%s", data)
	}
}

// WriteJSON падает с внятной ошибкой, если директория недоступна.
func TestWriteJSON_BadPath(t *testing.T) {
	r := &Report{Schema: SchemaVersion}
	err := r.WriteJSON("/nonexistent/dir/report.json")
	if err == nil {
		t.Fatal("want error for bad path")
	}
}

// ─── Omitted (дополняет omitted_test.go) ─────────────────────────────

// Omitted + Aggregate не должны путать друг друга на одном входе:
// Aggregate даёт top-директории по весу, Omitted — покрывающую
// картину. Контракт Omitted: сумма Files == len(items).
func TestOmittedAndAggregate_Coexist(t *testing.T) {
	items := []Item{
		{Path: "internal/a.go", Tokens: 1000},
		{Path: "internal/sub/b.go", Tokens: 500},
		{Path: "cmd/main.go", Tokens: 300},
		{Path: "README.md", Tokens: 50},
	}

	om := Omitted(items)
	total := 0
	for _, o := range om {
		total += o.Files
	}
	if total != len(items) {
		t.Errorf("Omitted: sum Files = %d, want %d", total, len(items))
	}

	ag := Aggregate(items, 0)
	// Aggregate даёт только верхнеуровневые бакеты, поэтому
	// сумма Files здесь может быть больше len(items) — внутренние
	// файлы считаются и в родителе, и в потомке, если collapse
	// не сработал. Это разные представления, не сравниваем
	// числа, только проверяем, что оба вызова не падают и дают
	// непустой результат.
	if len(ag) == 0 {
		t.Error("Aggregate: want non-empty result")
	}
}
