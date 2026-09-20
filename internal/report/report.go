// Package report строит диагностику переполнения контекста:
// где сидит вес и какие меры (с пересчитанным эффектом) втиснут
// набор в потолок. Структура данных первична; текст и JSON —
// её представления.
package report

import (
	"encoding/json"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/it1ro/cake/internal/tokens"
	"github.com/it1ro/cake/pkg/types"
)

// SchemaVersion — версия JSON-схемы отчёта (контракт для CI).
const SchemaVersion = 1

// Item — файл с весом в токенах.
type Item struct {
	Path   string
	Tokens int
}

// Bucket — агрегат по ключу (директория или расширение).
type Bucket struct {
	Key    string `json:"key"`
	Files  int    `json:"files"`
	Tokens int    `json:"tokens"`
}

// Measure — набор флагов, уменьшающий набор, с пересчитанной
// оценкой «после».
type Measure struct {
	Label string   `json:"label"`
	Flags []string `json:"flags,omitempty"`
	After int      `json:"after"`
	Fits  bool     `json:"fits"`
}

// Report — результат проверки лимита.
type Report struct {
	Schema   int       `json:"schema"`
	Mode     string    `json:"mode"`
	Limit    int       `json:"limit"`
	Reserve  int       `json:"reserve"`
	Ceiling  int       `json:"ceiling"`
	Estimate int       `json:"estimate"`
	Exact    bool      `json:"exact"`
	Dirs     []Bucket  `json:"dirs"`
	Exts     []Bucket  `json:"exts"`
	Measures []Measure `json:"measures"`
}

// Params — вход Build.
type Params struct {
	Entries        []types.FileEntry
	Weight         map[string]int // path → токены; nil → по Size
	OverheadTokens int
	Limit          int
	Reserve        int
	Ceiling        int
	Exact          bool
	Mode           string
}

// Build собирает отчёт. Веса — токены (единица лимита).
func Build(p Params) *Report {
	items := make([]Item, len(p.Entries))
	total := p.OverheadTokens
	for i, e := range p.Entries {
		w := tokens.EstimateSize(e.Size)
		if p.Weight != nil {
			if v, ok := p.Weight[e.Path]; ok {
				w = v
			}
		}
		items[i] = Item{Path: e.Path, Tokens: w}
		total += w
	}
	return &Report{
		Schema:   SchemaVersion,
		Mode:     p.Mode,
		Limit:    p.Limit,
		Reserve:  p.Reserve,
		Ceiling:  p.Ceiling,
		Estimate: total,
		Exact:    p.Exact,
		Dirs:     Aggregate(items, 5),
		Exts:     AggregateExt(items, 5),
		Measures: measures(p, items, total),
	}
}

// WriteJSON пишет отчёт в файл (schema: 1).
func (r *Report) WriteJSON(file string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

// ─── Агрегация (ncdu-style) ──────────────────────────────────────────

type dirNode struct {
	path     string
	tokens   int
	files    int
	children map[string]*dirNode
}

// Aggregate агрегирует вес по директориям в стиле ncdu: спуск от
// корня, пока один дочерний каталог держит строго больше половины
// веса родителя. Так vendor/k8s.io/api/ не разворачивается до файла,
// а internal/ с равномерным весом не дробится.
// Файлы в корне проекта идут в бакет "./".
func Aggregate(items []Item, top int) []Bucket {
	root := &dirNode{children: map[string]*dirNode{}}
	rootFiles, rootTokens := 0, 0

	for _, it := range items {
		parts := strings.Split(it.Path, "/")
		if len(parts) == 1 {
			rootFiles++
			rootTokens += it.Tokens
			continue
		}
		cur := root
		for i := 0; i < len(parts)-1; i++ {
			child := cur.children[parts[i]]
			if child == nil {
				child = &dirNode{
					path:     strings.Join(parts[:i+1], "/"),
					children: map[string]*dirNode{},
				}
				cur.children[parts[i]] = child
			}
			child.tokens += it.Tokens
			child.files++
			cur = child
		}
	}

	var out []Bucket
	if rootFiles > 0 {
		out = append(out, Bucket{Key: "./", Files: rootFiles, Tokens: rootTokens})
	}
	for _, c := range root.children {
		out = append(out, collapse(c))
	}
	return sortTrim(out, top)
}

func collapse(n *dirNode) Bucket {
	for {
		var dom *dirNode
		for _, c := range n.children {
			if c.tokens*2 <= n.tokens {
				continue
			}
			if dom == nil || c.tokens > dom.tokens ||
				(c.tokens == dom.tokens && c.path < dom.path) {
				dom = c
			}
		}
		if dom == nil {
			break
		}
		n = dom
	}
	return Bucket{Key: n.path + "/", Files: n.files, Tokens: n.tokens}
}

// AggregateExt агрегирует вес по расширениям.
func AggregateExt(items []Item, top int) []Bucket {
	m := map[string]*Bucket{}
	for _, it := range items {
		ext := strings.ToLower(path.Ext(it.Path))
		if ext == "" {
			ext = "(нет)"
		}
		b := m[ext]
		if b == nil {
			b = &Bucket{Key: ext}
			m[ext] = b
		}
		b.Files++
		b.Tokens += it.Tokens
	}
	out := make([]Bucket, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	return sortTrim(out, top)
}

func sortTrim(bs []Bucket, top int) []Bucket {
	sort.Slice(bs, func(i, j int) bool {
		if bs[i].Tokens != bs[j].Tokens {
			return bs[i].Tokens > bs[j].Tokens
		}
		return bs[i].Key < bs[j].Key
	})
	if top > 0 && len(bs) > top {
		bs = bs[:top]
	}
	return bs
}

// ─── Omitted (схлопывание отброшенного) ─────────────────────────────

// Omitted агрегирует отброшенные файлы в схлопнутые директории:
// каталог без собственных файлов и с ровно одним дочерним
// каталогом сливается с этим ребёнком. Так
//
//	vendor/k8s.io/api/types.go, vendor/k8s.io/api/more.go
//
// даёт один бакет "vendor/k8s.io/api/", а
//
//	internal/a.go, internal/b.go
//
// — бакет "internal/". Файлы в корне проекта идут в "./".
//
// Отличие от Aggregate: там правило «доминирующий ребёнок >50%»
// останавливается на первом разветвлении и теряет мелких соседей.
// Для Omitted нужна полная покрывающая картина: сумма Files по
// всем OmittedDir равна len(items). Это контракт, который
// проверяется тестом TestOmitted_CoversAllInput.
//
// Возвращает nil для пустого входа. Порядок — по токенам убыв.,
// при равенстве — по имени.
func Omitted(items []Item) []types.OmittedDir {
	if len(items) == 0 {
		return nil
	}

	// ownFiles/ownTokens — файлы, лежащие непосредственно в этом
	// каталоге (не в подкаталогах). Останавливают схлопывание:
	// «internal/x.go и internal/sub/y.go» не должны слиться
	// в "internal/sub/".
	//
	// subFiles/subTokens — суммарно по поддереву, включая этот
	// каталог. Нужны для итогового бакета, когда схлопывание
	// остановилось.
	type node struct {
		children  map[string]*node
		ownFiles  int
		ownTokens int
		subFiles  int
		subTokens int
	}
	root := &node{children: map[string]*node{}}

	for _, it := range items {
		parts := strings.Split(it.Path, "/")
		cur := root
		cur.subFiles++
		cur.subTokens += it.Tokens
		for i := 0; i < len(parts)-1; i++ {
			c := cur.children[parts[i]]
			if c == nil {
				c = &node{children: map[string]*node{}}
				cur.children[parts[i]] = c
			}
			c.subFiles++
			c.subTokens += it.Tokens
			cur = c
		}
		cur.ownFiles++
		cur.ownTokens += it.Tokens
	}

	var out []types.OmittedDir

	// Корневые файлы (README.md, go.mod, …) — отдельный бакет.
	if root.ownFiles > 0 {
		out = append(out, types.OmittedDir{
			Path:   "./",
			Files:  root.ownFiles,
			Tokens: root.ownTokens,
		})
	}

	// Каждый верхнеуровневый каталог — своя цепочка схлопывания.
	for name, c := range root.children {
		cur := c
		curPath := name
		// Идём вглубь, пока каталог «прозрачный»: без собственных
		// файлов и с единственным дочерним каталогом.
		for cur.ownFiles == 0 && len(cur.children) == 1 {
			for n, cc := range cur.children {
				curPath = curPath + "/" + n
				cur = cc
				break
			}
		}
		out = append(out, types.OmittedDir{
			Path:   curPath + "/",
			Files:  cur.subFiles,
			Tokens: cur.subTokens,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// ─── Меры ────────────────────────────────────────────────────────────

func measures(p Params, items []Item, total int) []Measure {
	if total <= p.Ceiling {
		return nil
	}
	var out []Measure
	if m, ok := excludeMeasure(p, items, total); ok {
		out = append(out, m)
	}
	if m, ok := includeMeasure(p, items, total); ok {
		out = append(out, m)
	}
	return out
}

// excludeMeasure жадно набирает excludes, пересчитывая эффект после
// каждого шага реальным doublestar.Match — вложенные пути не
// считаются дважды. --mode clean: см. review §4.3 (отложено на v0.3
// в этой итерации).
func excludeMeasure(p Params, items []Item, total int) (Measure, bool) {
	var cands []string
	seen := map[string]bool{}
	add := func(c string) {
		if !seen[c] {
			seen[c] = true
			cands = append(cands, c)
		}
	}

	for _, b := range Aggregate(items, 10) {
		if b.Key != "./" {
			add(b.Key + "**")
		}
	}
	for _, it := range items {
		if strings.HasSuffix(it.Path, "_test.go") {
			add("**/*_test.go")
			break
		}
	}
	// Главное расширение проекта не предлагаем к исключению целиком:
	// «выкинуть все .go» — не совет.
	for i, b := range AggregateExt(items, 5) {
		if i > 0 && b.Key != "(нет)" {
			add("**/*" + b.Key)
		}
	}

	alive := make([]bool, len(items))
	for i := range alive {
		alive[i] = true
	}
	used := map[string]bool{}
	remaining := total
	var flags []string

	for remaining > p.Ceiling {
		best, bestSaved := "", 0
		for _, c := range cands {
			if used[c] {
				continue
			}
			if s := saved(c, items, alive); s > bestSaved {
				best, bestSaved = c, s
			}
		}
		if bestSaved == 0 {
			break
		}
		used[best] = true
		for i, it := range items {
			if alive[i] {
				if ok, _ := doublestar.Match(best, it.Path); ok {
					alive[i] = false
				}
			}
		}
		remaining -= bestSaved
		flags = append(flags, "-e", best)
	}
	if len(flags) == 0 {
		return Measure{}, false
	}
	return Measure{
		Label: "exclude",
		Flags: flags,
		After: remaining,
		Fits:  remaining <= p.Ceiling,
	}, true
}

func saved(pattern string, items []Item, alive []bool) int {
	s := 0
	for i, it := range items {
		if !alive[i] {
			continue
		}
		if ok, _ := doublestar.Match(pattern, it.Path); ok {
			s += it.Tokens
		}
	}
	return s
}

func includeMeasure(p Params, items []Item, total int) (Measure, bool) {
	exts := AggregateExt(items, 1)
	if len(exts) == 0 || exts[0].Key == "(нет)" {
		return Measure{}, false
	}
	after := exts[0].Tokens + p.OverheadTokens
	if after >= total {
		return Measure{}, false
	}
	return Measure{
		Label: "include",
		Flags: []string{"-i", "**/*" + exts[0].Key},
		After: after,
		Fits:  after <= p.Ceiling,
	}, true
}
