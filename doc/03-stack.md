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
- `github.com/bmatcuk/doublestar/v4` — glob-паттерны с `**`.
- Свой парсер `.gitignore` (`internal/gitignore`) — `sabhiram/go-gitignore`
  не поддерживает вложенные `.gitignore` и dir-only правила. 250 строк,
  ноль зависимостей.

### Go-парсинг
- std `go/parser`, `go/ast`, `go/token`, `go/printer`.
- (опц.) `golang.org/x/tools/imports` — нормализация импортов.

### TUI
- `github.com/charmbracelet/bubbletea` — event loop.
- `github.com/charmbracelet/lipgloss` — стили.
- `github.com/sahilm/fuzzy` — fuzzy-поиск в flat-режиме.

### Конфиг
- `github.com/pelletier/go-toml/v2` — разбор `cake.toml`, strict-режим,
  без CGo и без транзитивных зависимостей.

### TTY
- `github.com/mattn/go-isatty` — определение терминала для цветов
  summary; `IsCygwinTerminal` для mintty / Git Bash. Раньше был
  косвенной зависимостью через bubbletea; с PR 6.1 — прямая.

### Токены
- v0.x: собственная эвристика `len/4`.
- v0.2+: `github.com/tiktoken-go/tokenizer` (WASM-биндинги, без CGo).

### Буфер обмена
- Приоритет: OSC 52 через `/dev/tty` (Unix) / `CONOUT$` (Windows),
  работает без зависимостей и cgo.
- Fallback: `golang.design/x/clipboard` (требует cgo на части платформ) —
  не используем.

### Тесты
- std `testing`, `go test -race`.
- `github.com/stretchr/testify` — только для assertions в сложных местах.
- `testdata/` для фикстур парсинга.

## Инструменты разработки

- `make` — таск-раннер.
- `goreleaser` — сборка релизов.
- `golangci-lint` — линт.
- `pre-commit` (опц.) — хуки на fmt/lint.

## Структура репо

```
cake/
├── cmd/cake/           # main
├── internal/
│   ├── cli/           # cobra-команды + applyFlags
│   ├── walker/
│   ├── processor/
│   │   └── golang/    # GoProcessor
│   ├── render/
│   ├── tui/
│   ├── pipeline/      # + overflow.go, summary.go
│   ├── config/        # cake.toml
│   ├── report/        # диагностика переполнения
│   ├── tokens/
│   └── clipboard/
├── pkg/types/
├── doc/
│   ├── adr/
│   └── journal.md
└── Makefile
```

## Что сознательно НЕ используем

- **CGo** — до v1.0. Ломает простые бинари.
- **Tree-sitter** в v0.x — сложность интеграции без CGo.
- **Reflection** — не нужно.
- **Heavyweight DI** (wire, dig) — ручное конструирование читаемее на этом масштабе.
- **Встроенная таблица моделей** (`claude-200k`, `gpt4-128k` и т.п.) —
  устареет за квартал; неверная цифра хуже отсутствующей.
  Пользователь задаёт лимит сам или в `cake.toml`.
```

---

## `doc/04-roadmap.md`

```markdown
# Roadmap

## v0.1 — MVP (Go-centric) · ~2 недели

- [x] Скелет репо, CI, goreleaser
- [x] `internal/walker` с поддержкой `.gitignore`, include/exclude
- [x] `internal/render/xml` + стриминг
- [x] `cmd/dump` — UC-1
- [x] `internal/processor/golang` — strip comments + doc
- [x] `cmd/clean` — UC-2
- [x] `internal/tui` — базовое дерево, fuzzy, select, export
- [x] `cmd/pick` — UC-3
- [x] Тесты на walker / processor / render
- [x] README + примеры

## v0.2 — Полировка UX · ~1 неделя

- [x] `--budget` — обрезка по лимиту
- [x] OSC 52 clipboard
- [x] Markdown и plain рендеры
- [x] Бенчмарки на больших репо
- [x] Эвристика `len/4` вместо tiktoken (в v0.3)
- [x] **Лимит контекста:** `--context-limit`, `--reserve`,
      `--on-overflow=fail|drop`, `--profile`, `cake.toml`,
      отчёт о переполнении, коды выхода 0/1/3
- [x] Учёт обвязки формата в оценке токенов (`render.Overhead`)
- [x] `Context.Omitted` — схлопнутые директории отброшенного в `<tree>`
- [x] TUI-индикатор лимита в статусной строке
- [x] Предупреждение про OSC 52 в summary при выводе > 100 KB
- [ ] Прогресс-бар в CLI

## v0.3 — Мультиязычность и точность

- [ ] Интерфейс `Processor` вынесен в `pkg/`
- [ ] `Passthrough` для не-Go файлов
- [ ] Regex-based комментарии для Python/JS/Rust (без AST)
- [ ] Tree-sitter через `malivvan/tree-sitter` (WASM)
- [ ] tiktoken (точная оценка токенов) — отдельный ADR
- [ ] `cake explain <path>` — почему файл попал или не попал
- [ ] Приоритетное отсечение (generated → тесты → крупные)
- [ ] Интерактивный `suggest` в dump/clean

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
