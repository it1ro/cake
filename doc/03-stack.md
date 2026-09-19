# Stack

## Язык и тулчейн

- **Go 1.22+** (нужен `range-over-func`, нормальные generics, улучшенный `slices`/`maps` в std).
- Модули без `vendor/` на старте.
- `golangci-lint` для CI.

## Зависимости (v0.x)

### CLI
- `github.com/spf13/cobra` — команды и флаги.

### Обход ФС
- std `io/fs`, `path/filepath` — основа.
- `github.com/sabhiram/go-gitignore` — `.gitignore`-правила.
- `github.com/bmatcuk/doublestar/v4` — glob-паттерны с `**`.

### Go-парсинг
- std `go/parser`, `go/ast`, `go/token`, `go/printer`.
- (опц.) `golang.org/x/tools/imports` — нормализация импортов.

### TUI
- `github.com/charmbracelet/bubbletea` — event loop.
- `github.com/charmbracelet/bubbles` — готовые компоненты (list, textinput, viewport).
- `github.com/charmbracelet/lipgloss` — стили.
- `github.com/sahilm/fuzzy` — fuzzy-поиск.

### Токены
- v0.x: собственная эвристика.
- v0.2+: `github.com/tiktoken-go/tokenizer` (WASM-биндинги, без CGo).

### Буфер обмена
- Приоритет: OSC 52 через stdout (работает без зависимостей).
- Fallback: `golang.design/x/clipboard` (требует cgo на части платформ).

### Тесты
- std `testing`, `go test -race`.
- `github.com/stretchr/testify` — только для assertions в сложных местах.
- `testdata/` для фикстур парсинга.

## Инструменты разработки

- `just` (или `make`) — таск-раннер.
- `goreleaser` — сборка релизов.
- `golangci-lint` — линт.
- `pre-commit` (опц.) — хуки на fmt/lint.

## Структура репо

```
cake/
├── cmd/cake/           # main
├── internal/
│   ├── cli/           # cobra-команды
│   ├── walker/
│   ├── processor/
│   │   └── golang/    # GoProcessor
│   ├── render/
│   ├── tui/
│   ├── pipeline/
│   └── tokens/
├── pkg/types/
├── docs/
├── testdata/
└── justfile
```

## Что сознательно НЕ используем

- **CGo** — до v1.0. Ломает простые бинари.
- **Tree-sitter** в v0.x — сложность интеграции без CGo.
- **Reflection** — не нужно.
- **Heavyweight DI** (wire, dig) — ручное конструирование читаемее на этом масштабе.
