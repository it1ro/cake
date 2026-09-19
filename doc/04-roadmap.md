# Roadmap

## v0.1 — MVP (Go-centric) · ~2 недели

- [x] Скелет репо, CI, goreleaser
- [ ] `internal/walker` с поддержкой `.gitignore`, include/exclude
- [ ] `internal/render/xml` + стриминг
- [ ] `cmd/dump` — UC-1
- [ ] `internal/processor/golang` — strip comments + doc
- [ ] `cmd/clean` — UC-2
- [ ] `internal/tui` — базовое дерево, fuzzy, select, export
- [ ] `cmd/pick` — UC-3
- [ ] Тесты на walker / processor / render
- [ ] README + примеры

## v0.2 — Полировка UX · ~1 неделя

- [ ] Оценка токенов (tiktoken WASM)
- [ ] `--budget` — обрезка по лимиту
- [ ] OSC 52 clipboard
- [ ] Markdown и plain рендеры
- [ ] Прогресс-бар в CLI
- [ ] Бенчмарки на больших репо

## v0.3 — Мультиязычность

- [ ] Интерфейс `Processor` вынесен в `pkg/`
- [ ] `Passthrough` для не-Go файлов
- [ ] Regex-based комментарии для Python/JS/Rust (без AST)
- [ ] Tree-sitter через `malivvan/tree-sitter` (WASM)

## v0.4 — Расширения

- [ ] MCP-сервер (stdio)
- [ ] `--skeleton` режим (только сигнатуры)
- [ ] Custom templates для рендера
- [ ] Плагины через `plugin` или subprocess

## v1.0

- [ ] Стабильный API `pkg/`
- [ ] Покрытие ≥ 80%
- [ ] Документация на сайте
- [ ] Homebrew / scoop / apt пакеты
```

---

## `docs/journal.md`

```markdown
# Journal

Дневник разработки. Записи в обратном хронологическом порядке.
Формат: `YYYY-MM-DD — заголовок`, затем контекст, решения, вопросы.

---

## 2026-09-20 — Старт проекта

### Что сделано
- Определены три use case MVP: dump, clean, pick.
- Зафиксирована архитектура (см. architecture.md).
- Выбран стек без CGo.

### Решения
- **Один pipeline, три команды.** Не дублировать логику в TUI и CLI.
- **Без tree-sitter на старте.** `go/ast` даёт 100% точность для Go.
- **XML как формат по умолчанию.** LLM стабильно его парсят; теги
  чётко разделяют метаданные и содержимое.
- **OSC 52 в приоритете над cgo-буфером.** Не ломает статическую сборку.

### Открытые вопросы
- Оценка токенов: эвристика `/4` или сразу tiktoken? Решение: эвристика
  в v0.1, tiktoken — опция во v0.2.
- Как обрабатывать файлы с build-тегами? Пока — включать всё, помечать
  в метаданных.
- Формат дерева: свой или через `github.com/disintegration/imaging`?
  Свой — проще контролировать префиксы `├──`/`└──`.

### Следующие шаги
1. `go mod init`.
2. Скелет `cmd/cake` + cobra root.
3. `internal/walker` с базовым обходом (без gitignore).
4. Ручной smoke-тест на этом репо.
```

---

# 🚀 Приступаем

Давай сделаю первый коммит: структура + рабочая команда `dump` без gitignore (добавим чуть позже).

## Шаг 1. Инициализация

```bash
mkdir cake && cd cake
go mod init github.com/it1ro/cake
go get github.com/spf13/cobra@latest
go get github.com/bmatcuk/doublestar/v4@latest
```

Структура:

```
cake/
├── cmd/cake/main.go
├── internal/
│   ├── cli/root.go
│   ├── cli/dump.go
│   ├── walker/walker.go
│   ├── render/xml.go
│   └── pipeline/pipeline.go
├── pkg/types/types.go
├── docs/                  # ← файлы, что выше
├── go.mod
└── go.sum
```

## Шаг 2. `pkg/types/types.go`

```go
package types

// FileEntry — файл, найденный walker'ом.
type FileEntry struct {
	Path     string // относительный путь от корня
	AbsPath  string
	Size     int64
	Language string
}

// ProcessedFile — файл после processor'а, готов к рендеру.
type ProcessedFile struct {
	Entry   FileEntry
	Content []byte
	Lines   int
}

// Context — всё, что уходит в render.
type Context struct {
	Project string
	Root    string
	Files   []ProcessedFile
	Tokens  int
}
```

## Шаг 3. `internal/walker/walker.go`

```go
package walker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/it1ro/cake/pkg/types"
)

type Options struct {
	Root     string
	Includes []string // glob-паттерны; пусто = всё
	Excludes []string // glob-паттерны
	MaxSize  int64    // байт; 0 = без лимита
}

func Walk(opts Options) ([]types.FileEntry, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}

	var entries []types.FileEntry

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Пропускаем .git и подобное
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		if !matches(rel, opts.Includes, opts.Excludes) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if opts.MaxSize > 0 && info.Size() > opts.MaxSize {
			return nil
		}
		if isBinary(path) {
			return nil
		}

		entries = append(entries, types.FileEntry{
			Path:     rel,
			AbsPath:  path,
			Size:     info.Size(),
			Language: detectLanguage(path),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func matches(rel string, includes, excludes []string) bool {
	for _, ex := range excludes {
		if ok, _ := doublestar.Match(ex, rel); ok {
			return false
		}
	}
	if len(includes) == 0 {
		return true
	}
	for _, inc := range includes {
		if ok, _ := doublestar.Match(inc, rel); ok {
			return true
		}
	}
	return false
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

var langByExt = map[string]string{
	".go":   "go",
	".md":   "markdown",
	".yaml": "yaml",
	".yml":  "yaml",
	".json": "json",
	".toml": "toml",
	".sh":   "shell",
	".py":   "python",
	".rs":   "rust",
	".js":   "javascript",
	".ts":   "typescript",
}

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := langByExt[ext]; ok {
		return lang
	}
	return "text"
}

// Tree строит ASCII-дерево из списка путей (для рендера).
func Tree(entries []types.FileEntry) string {
	// ... реализуем в отдельном коммите
	return fmt.Sprintf("(%d files)", len(entries))
}
```

## Шаг 4. `internal/render/xml.go`

```go
package render

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/it1ro/cake/pkg/types"
)

// XML рендерит Context в XML-подобный формат.
// Стриминг: пишем по одному файлу, не буферизуем весь вывод.
func XML(cake types.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintf(bw, `<context project=%q root=%q files=%d tokens=%d>`+"\n",
		cake.Project, cake.Root, len(cake.Files), cake.Tokens)

	// Дерево
	bw.WriteString("  <tree><![CDATA[\n")
	bw.WriteString(indent(buildTree(cake.Files), "    "))
	bw.WriteString("  ]]></tree>\n\n")

	for _, f := range cake.Files {
		fmt.Fprintf(bw, "  <file path=%q lang=%q lines=%d bytes=%d>\n",
			f.Entry.Path, f.Entry.Language, f.Lines, len(f.Content))
		bw.WriteString("  <![CDATA[\n")

		// Экранируем ]]> внутри содержимого, иначе CDATA сломается
		content := strings.ReplaceAll(string(f.Content), "]]>", "]]]]><![CDATA[>")
		bw.WriteString(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			bw.WriteByte('\n')
		}
		bw.WriteString("  ]]>\n")
		bw.WriteString("  </file>\n\n")
	}

	bw.WriteString("</context>\n")
	return nil
}

func buildTree(files []types.ProcessedFile) string {
	// временная заглушка, заменим на нормальное дерево
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(f.Entry.Path)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}
```

## Шаг 5. `internal/pipeline/pipeline.go`

```go
package pipeline

import (
	"os"

	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/internal/walker"
	"github.com/it1ro/cake/pkg/types"
)

type Options struct {
	Root     string
	Includes []string
	Excludes []string
	MaxSize  int64
	Output   string // путь к файлу или "" = stdout
}

func Run(opts Options) error {
	entries, err := walker.Walk(walker.Options{
		Root:     opts.Root,
		Includes: opts.Includes,
		Excludes: opts.Excludes,
		MaxSize:  opts.MaxSize,
	})
	if err != nil {
		return err
	}

	processed := make([]types.ProcessedFile, 0, len(entries))
	for _, e := range entries {
		content, err := os.ReadFile(e.AbsPath)
		if err != nil {
			continue
		}
		processed = append(processed, types.ProcessedFile{
			Entry:   e,
			Content: content,
			Lines:   countLines(content),
		})
	}

	cake := types.Context{
		Project: opts.Root,
		Root:    opts.Root,
		Files:   processed,
	}

	out := os.Stdout
	if opts.Output != "" {
		f, err := os.Create(opts.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	return render.XML(cake, out)
}

func countLines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}
```

## Шаг 6. `internal/cli/dump.go` + `root.go`

```go
// internal/cli/dump.go
package cli

import (
	"github.com/spf13/cobra"
	"github.com/it1ro/cake/internal/pipeline"
)

var dumpOpts pipeline.Options

var dumpCmd = &cobra.Command{
	Use:   "dump [path]",
	Short: "Полный тегированный дамп проекта",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			dumpOpts.Root = args[0]
		}
		if dumpOpts.Root == "" {
			dumpOpts.Root = "."
		}
		return pipeline.Run(dumpOpts)
	},
}

func init() {
	dumpCmd.Flags().StringSliceVarP(&dumpOpts.Includes, "include", "i", nil, "glob-паттерны для включения")
	dumpCmd.Flags().StringSliceVarP(&dumpOpts.Excludes, "exclude", "e", nil, "glob-паттерны для исключения")
	dumpCmd.Flags().Int64Var(&dumpOpts.MaxSize, "max-size", 1<<20, "макс. размер файла в байтах")
	dumpCmd.Flags().StringVarP(&dumpOpts.Output, "output", "o", "", "файл вывода (по умолчанию stdout)")
	rootCmd.AddCommand(dumpCmd)
}
```

```go
// internal/cli/root.go
package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "cake",
	Short: "Извлекает контекст из кодовой базы для LLM",
}

func Execute() error { return rootCmd.Execute() }
```

```go
// cmd/cake/main.go
package main

import (
	"fmt"
	"os"

	"github.com/it1ro/cake/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

## Шаг 7. Smoke-тест

```bash
go build ./...
go run ./cmd/cake dump . -i "**/*.go" -o /tmp/dump.xml
head -50 /tmp/dump.xml
```

---

## Что дальше (задача на следующий заход)

1. Нормальный `walker.Tree` — рисовать `├── / └──` по отсортированным путям.
2. `internal/processor/golang` — парсинг и strip comments.
3. `cmd/clean`.
4. `.gitignore` через `sabhiram/go-gitignore`.
5. Первый коммит в git с этими пятью документами в `docs/`.
