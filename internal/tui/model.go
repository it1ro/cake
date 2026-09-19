// Package tui реализует интерактивный выбор файлов для команды
// cake pick. Модель получает список FileEntry (от pipeline.Plan),
// даёт пользователю отредактировать выбор и возвращает готовый
// набор через SelectedFiles().
//
// Модель не читает содержимое файлов и не рендерит вывод —
// этим занимается pipeline.RunWith после выхода из TUI.
package tui

import (
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/internal/tokens"
	"github.com/it1ro/cake/pkg/types"
)

// Model — состояние TUI. Значимый тип: Update возвращает копию,
// как принято в bubbletea.
type Model struct {
	// Данные
	all      []types.FileEntry // полный список (из Plan)
	visible  []types.FileEntry // после fuzzy-фильтра
	selected map[string]bool   // path → выбран

	// Курсор и фильтр
	cursor   int
	filter   string
	inFilter bool // режим ввода фильтра (символы идут в filter, не в команды)

	// Опции вывода; Mode и Format редактируются в TUI
	opts pipeline.Options

	// Терминал
	width, height int

	// Результат
	confirmed bool // пользователь нажал enter — RunWith вызывается
}

// New создаёт модель со списком файлов и базовыми опциями
// (обычно из cli/pick.go — флаги --format, --output и т.п.).
func New(all []types.FileEntry, opts pipeline.Options) Model {
	return Model{
		all:      all,
		visible:  all,
		selected: make(map[string]bool),
		opts:     opts,
	}
}

// Init — требование tea.Model. Ничего не запускаем.
func (m Model) Init() tea.Cmd { return nil }

// Update — требование tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.inFilter {
			return m.updateFilter(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
	case "G":
		if len(m.visible) > 0 {
			m.cursor = len(m.visible) - 1
		}

	case " ":
		m.toggleCursor()

	case "a":
		for _, e := range m.visible {
			m.selected[e.Path] = true
		}
	case "A":
		for _, e := range m.visible {
			delete(m.selected, e.Path)
		}

	case "d": // toggle всей директории текущего файла
		m.toggleDir()

	case "/":
		m.inFilter = true

	case "tab":
		if m.opts.Mode == pipeline.ModeDump {
			m.opts.Mode = pipeline.ModeClean
		} else {
			m.opts.Mode = pipeline.ModeDump
		}

	case "f":
		switch m.opts.Format {
		case render.FormatXML:
			m.opts.Format = render.FormatMarkdown
		case render.FormatMarkdown:
			m.opts.Format = render.FormatPlain
		default:
			m.opts.Format = render.FormatXML
		}

	case "enter":
		m.confirmed = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc":
		m.inFilter = false
		m.filter = ""
	case "enter":
		m.inFilter = false
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	default:
		// Печатаемые символы. msg.String() для обычных клавиш
		// возвращает одну руну; для спецклавиш — "up", "ctrl+x" и т.п.
		if len(s) == 1 {
			m.filter += s
		}
	}
	m.visible = filterEntries(m.all, m.filter)
	if m.cursor >= len(m.visible) {
		m.cursor = 0
	}
	return m, nil
}

// ─── Операции над выбором ────────────────────────────────────────────

func (m *Model) toggleCursor() {
	if len(m.visible) == 0 {
		return
	}
	p := m.visible[m.cursor].Path
	if m.selected[p] {
		delete(m.selected, p)
	} else {
		m.selected[p] = true
	}
}

// toggleDir: если все файлы директории текущего файла выбраны —
// снять; иначе — выбрать все.
func (m *Model) toggleDir() {
	if len(m.visible) == 0 {
		return
	}
	cur := m.visible[m.cursor]
	dir := path.Dir(cur.Path)
	prefix := ""
	if dir != "." {
		prefix = dir + "/"
	}

	allSelected := true
	any := false
	for _, e := range m.all {
		if !strings.HasPrefix(e.Path, prefix) {
			continue
		}
		any = true
		if !m.selected[e.Path] {
			allSelected = false
			break
		}
	}
	if !any {
		return
	}

	for _, e := range m.all {
		if strings.HasPrefix(e.Path, prefix) {
			m.selected[e.Path] = !allSelected
		}
	}
}

// ─── Результаты для cli/pick.go ──────────────────────────────────────

// Confirmed сообщает, нажал ли пользователь enter.
func (m Model) Confirmed() bool { return m.confirmed }

// SelectedFiles возвращает выбранные файлы в исходном порядке
// (отсортированном Plan'ом) — важно для детерминизма вывода.
func (m Model) SelectedFiles() []types.FileEntry {
	out := make([]types.FileEntry, 0, len(m.selected))
	for _, e := range m.all {
		if m.selected[e.Path] {
			out = append(out, e)
		}
	}
	return out
}

// Options возвращает опции с учётом переключений Mode/Format в TUI.
func (m Model) Options() pipeline.Options { return m.opts }

// SelectedCount и SelectedTokens — для отрисовки статуса.
func (m Model) SelectedCount() int { return len(m.selected) }

func (m Model) SelectedTokens() int {
	total := 0
	for _, e := range m.all {
		if m.selected[e.Path] {
			total += tokens.EstimateSize(e.Size)
		}
	}
	return total
}

// ─── Фильтр ──────────────────────────────────────────────────────────

func filterEntries(all []types.FileEntry, pattern string) []types.FileEntry {
	if pattern == "" {
		return all
	}
	paths := make([]string, len(all))
	for i, e := range all {
		paths[i] = e.Path
	}
	matches := fuzzy.Find(pattern, paths)
	out := make([]types.FileEntry, len(matches))
	for i, mt := range matches {
		out[i] = all[mt.Index]
	}
	return out
}
