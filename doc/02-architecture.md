# Architecture

## Общая схема

                    ┌─────────────────┐
                    │   CLI (cobra)   │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
         cmd/dump       cmd/clean       cmd/pick
              │              │              │
              └──────────────┼──────────────┘
                             ▼
                    ┌─────────────────┐
                    │    pipeline     │
                    └────────┬────────┘
                             │
        ┌────────┬───────────┼───────────┬────────┐
        ▼        ▼           ▼           ▼        ▼
     walker  processor   render      sink    tokens
```

## Пакеты

### `internal/walker`
Отвечает за обход ФС и фильтрацию.

- `Walk(root string, opts Options) ([]FileEntry, error)`
- Учитывает `.gitignore` (корневой + вложенные).
- Исключает: `.git/`, бинарные файлы, симлинки с зацикливанием.
- Возвращает отсортированный список для детерминированного вывода.

### `internal/processor`
Интерфейс мультиязычной обработки — задел под расширение.

```go
type Processor interface {
    Supports(path string) bool
    Language(path string) string
    Process(path string, src []byte, opts Options) (Result, error)
}
```

Реализации в v0.x:
- `GoProcessor` — парсит через `go/ast`, умеет strips-comments.
- `Passthrough` — отдаёт файл как есть.

Реестр — срез `[]Processor`, поиск `For(path)`.

### `internal/render`
Форматирование результата.

- `xml.go` — тегированный вывод (основной).
- `markdown.go` — заголовки + code-блоки.
- `plain.go` — для отладки.

Вход — `Context` (метаданные + список обработанных файлов),
выход — `io.Writer`.

### `internal/tui`
Bubbletea-модель поверх `pipeline`.

- Дерево файлов с чекбоксами.
- Состояние: `selected map[string]bool`, `cursor int`, `filter string`.
- Live-preview токенов.
- На `enter` — вызывает `pipeline.Run`.

### `internal/pipeline`
Оркестрация. Единственное место, где встречаются все слои.

```go
type Options struct {
    Root     string
    Includes []string
    Excludes []string
    Mode     Mode        // Dump | Clean
    Format   Format      // XML | Markdown | Plain
    Sink     Sink        // Stdout | File | Clipboard
    Budget   int         // токенов, 0 = без лимита
}

func Run(opts Options) error
```

### `internal/tokens`
Оценка токенов.

- `Estimate(src []byte) int` — быстрая эвристика (`len/4`).
- Позже — `tiktoken` как опция.

### `pkg/types`
Общие типы: `FileEntry`, `Context`, `ProcessedFile`.

## Формат вывода (v0.1)

```xml
<context project="myproject" files="42" tokens="18432">
  <tree><![CDATA[
  ├── cmd/
  │   └── main.go
  └── internal/walker/walker.go
  ]]></tree>

  <file path="cmd/main.go" lang="go" lines="24" bytes="512">
  <![CDATA[
package main
...
  ]]>
  </file>
</context>
```

## Поток данных

1. `cmd/*` парсит флаги → `pipeline.Options`.
2. `walker.Walk` → `[]FileEntry`.
3. Для каждого `FileEntry` → `processor.For(path)` → `Process`.
4. Собранный `Context` → `render.Render` → `io.Writer` (sink).
5. Sink: stdout / файл / OSC 52 в терминал.

TUI-режим отличается только источником списка файлов:
вместо `walker.Walk` — модель `tui`, которую пользователь редактирует.

## Принципы

- **Один конвейер, три входа.** TUI и CLI не расходятся.
- **Стриминг.** Не буферизуем больше одного файла за раз (кроме tiny preview).
- **Детерминизм.** Сортировка по пути, стабильный вывод — важно для diff.
- **Ноль глобального состояния.** Все зависимости — через опции.
