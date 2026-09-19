// Package processor определяет интерфейс обработки файлов
// и реестр процессоров. Реализации живут в подпакетах
// (golang, python, …) и регистрируются через init().
package processor

import (
	"path/filepath"
	"strings"
)

// Options — общие опции для всех процессоров.
type Options struct {
	// KeepDoc сохраняет doc-комментарии (Go: комментарии перед
	// объявлениями). Build-констрейнты и //go:* директивы
	// сохраняются всегда.
	KeepDoc bool
}

// Processor обрабатывает файл одного языка.
type Processor interface {
	// Supports сообщает, может ли процессор работать с этим путём.
	Supports(path string) bool
	// Language возвращает короткий идентификатор языка (для тега lang).
	Language(path string) string
	// Process трансформирует содержимое. При ошибке парсинга
	// может вернуть исходные данные без ошибки — вызывающая сторона
	// решает, как реагировать.
	Process(path string, src []byte, opts Options) ([]byte, error)
}

var registry []Processor

// Register добавляет процессор в реестр. Вызывается из init().
// Порядок важен: первые зарегистрированные проверяются первыми.
func Register(p Processor) {
	registry = append(registry, p)
}

// For возвращает процессор для файла. Никогда не nil —
// если ничего не подошло, вернётся passthrough.
func For(path string) Processor {
	for _, p := range registry {
		if p.Supports(path) {
			return p
		}
	}
	return passthrough{}
}

// Language определяет язык по расширению. Используется walker'ом
// и рендером — единая точка правды.
func Language(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := langByExt[ext]; ok {
		return lang
	}
	return "text"
}

var langByExt = map[string]string{
	".go":    "go",
	".md":    "markdown",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".toml":  "toml",
	".sh":    "shell",
	".bash":  "shell",
	".py":    "python",
	".rs":    "rust",
	".js":    "javascript",
	".ts":    "typescript",
	".tsx":   "typescript",
	".jsx":   "javascript",
	".sql":   "sql",
	".proto": "protobuf",
	".html":  "html",
	".css":   "css",
}

// passthrough — фолбэк: отдаёт файл как есть.
type passthrough struct{}

func (passthrough) Supports(string) bool     { return true }
func (passthrough) Language(p string) string { return Language(p) }
func (passthrough) Process(_ string, src []byte, _ Options) ([]byte, error) {
	return src, nil
}
