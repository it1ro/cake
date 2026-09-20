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
	root     *Node             // построено из all; используется в tree-режиме
	visible  []types.FileEntry // flat-режим, после fuzzy-фильтра
	flat     []flatItem        // tree-режим, после фильтра
	selected map[string]bool   // path → выбран

	// Курсор и фильтр
	cursor   int
	offset   int // верхняя видимая строка; меняется только когда курсор выходит за окно
	filter   string
	inFilter bool // режим ввода фильтра (символы идут в filter, не в команды)
	treeMode bool // true — tree, false — flat

	// Опции вывода; Mode и Format редактируются в TUI
	opts pipeline.Options

	// Терминал
	width, height int

	// Результат
	confirmed bool // пользователь нажал enter — RunWith вызывается
}

// New создаёт модель со списком файлов и базовыми опциями
// (обычно из cli/pick.go — флаги --format, --output и т.п.).
//
// Стартовый режим — flat. Для старта в дереве: New(...).WithTree().
func New(all []types.FileEntry, opts pipeline.Options) Model {
	return Model{
		all:      all,
		root:     buildTree(all),
		visible:  all,
		selected: make(map[string]bool),
		opts:     opts,
		treeMode: false,
	}
}

// WithTree переключает модель в древовидный режим до старта
// bubbletea-цикла. Вызывается один раз при конструировании
// (cli/pick.go); после — режим меняется хоткеем `t`.
func (m Model) WithTree() Model {
	m.treeMode = true
	m.rebuild()
	return m
}

// Init — требование tea.Model. Ничего не запускаем.
func (m Model) Init() tea.Cmd { return nil }

// Update — требование tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Высота окна изменилась — пересчитать видимую область,
		// иначе курсор может оказаться «за кадром».
		m.ensureCursorVisible()
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
	key := msg.String()
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < m.currentLen()-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
	case "G":
		if n := m.currentLen(); n > 0 {
			m.cursor = n - 1
		}

	case " ":
		m.toggleCursor()

	case "a":
		m.selectAllVisible(true)
	case "A":
		m.selectAllVisible(false)

	case "d":
		m.toggleDirScope()

	case "/":
		m.inFilter = true

	case "t":
		// toggle tree/list, сохраняя курсор на текущем файле
		prevPath := m.currentPath()
		m.treeMode = !m.treeMode
		m.rebuild()
		m.moveCursorToPath(prevPath)

	case "right", "l":
		if m.treeMode {
			if it := m.currentTreeItem(); it != nil && it.node.IsDir && !it.node.Expanded {
				prevPath := it.node.Path
				it.node.Expanded = true
				m.rebuild()
				m.moveCursorToPath(prevPath)
			}
		}

	case "left", "h":
		if m.treeMode {
			if it := m.currentTreeItem(); it != nil && it.node.IsDir && it.node.Expanded {
				prevPath := it.node.Path
				it.node.Expanded = false
				m.rebuild()
				m.moveCursorToPath(prevPath)
			} else if it := m.currentTreeItem(); it != nil && it.node.Parent != nil {
				// переход к родителю
				if it.node.Parent.Path != "" {
					m.moveCursorToPath(it.node.Parent.Path)
				}
			}
		}

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
	// Любое изменение курсора (или его видимого окружения)
	// должно скорректировать offset — и только здесь. Без этого
	// список «прилипал» курсором к нижнему краю при движении
	// вверх (см. renderList в view.go).
	m.ensureCursorVisible()
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
	m.rebuild()
	m.ensureCursorVisible()
	return m, nil
}

// ─── Пересборка видимого списка ──────────────────────────────────────

// rebuild пересчитывает visible (flat) или flat (tree) по текущему
// фильтру и корректирует курсор.
func (m *Model) rebuild() {
	if m.treeMode {
		m.flat = flatten(m.root, m.filter)
	} else {
		m.visible = filterEntries(m.all, m.filter)
	}
	n := m.currentLen()
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
}

// ensureCursorVisible держит cursor внутри видимого окна,
// двигая только offset. Курсор не «прилипает» к краю:
//
//   - движение вниз: пока курсор внутри окна — offset стоит;
//     когда курсор уходит за нижнюю границу — offset сдвигается
//     на одну строку (курсор остаётся внизу окна).
//   - движение вверх: пока курсор внутри окна — offset стоит;
//     когда курсор уходит за верхнюю границу — offset
//     подтягивается к курсору (курсор остаётся вверху окна).
//
// Работает для обоих режимов (flat и tree) — currentLen()
// знает, какой список активен.
func (m *Model) ensureCursorVisible() {
	height := m.listHeight()
	if height < 1 {
		height = 1
	}

	n := m.currentLen()
	if n == 0 {
		m.offset = 0
		return
	}

	// Курсор выше окна — подтянуть окно вверх.
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	// Курсор ниже окна — сдвинуть окно вниз ровно настолько,
	// чтобы курсор оказался на последней видимой строке.
	if m.cursor >= m.offset+height {
		m.offset = m.cursor - height + 1
	}

	// Не показывать пустое место внизу: если список короче,
	// чем позволяет окно, offset должен упираться в 0.
	maxOffset := n - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) currentLen() int {
	if m.treeMode {
		return len(m.flat)
	}
	return len(m.visible)
}

// currentPath возвращает путь текущего узла в любом режиме; ""
// если список пуст или корень (в дереве).
func (m Model) currentPath() string {
	if m.treeMode {
		if m.cursor < len(m.flat) {
			return m.flat[m.cursor].node.Path
		}
		return ""
	}
	if m.cursor < len(m.visible) {
		return m.visible[m.cursor].Path
	}
	return ""
}

func (m Model) currentTreeItem() *flatItem {
	if !m.treeMode || m.cursor >= len(m.flat) {
		return nil
	}
	return &m.flat[m.cursor]
}

func (m Model) currentFlatEntry() *types.FileEntry {
	if m.treeMode || m.cursor >= len(m.visible) {
		return nil
	}
	return &m.visible[m.cursor]
}

// moveCursorToPath пытается поставить курсор на узел с данным
// путём. Если узел больше не виден — оставляет курсор в границах.
func (m *Model) moveCursorToPath(p string) {
	if p == "" {
		return
	}
	if m.treeMode {
		for i, it := range m.flat {
			if it.node.Path == p {
				m.cursor = i
				return
			}
		}
	} else {
		for i, e := range m.visible {
			if e.Path == p {
				m.cursor = i
				return
			}
		}
	}
	if n := m.currentLen(); n > 0 && m.cursor >= n {
		m.cursor = n - 1
	}
}

// ─── Операции над выбором ────────────────────────────────────────────

// toggleCursor — space. Файл: инвертировать. Директория: если всё
// выбрано — снять, иначе — выбрать всё поддерево.
func (m *Model) toggleCursor() {
	if m.treeMode {
		it := m.currentTreeItem()
		if it == nil {
			return
		}
		if it.node.IsDir {
			m.toggleSubtree(it.node)
		} else {
			toggleFile(m.selected, it.node.Path)
		}
		return
	}
	if e := m.currentFlatEntry(); e != nil {
		toggleFile(m.selected, e.Path)
	}
}

func (m *Model) toggleSubtree(n *Node) {
	files := n.fileDescendants()
	sel, total := selectState(n, m.selected)
	all := total > 0 && sel == total
	for _, p := range files {
		if all {
			delete(m.selected, p)
		} else {
			m.selected[p] = true
		}
	}
}

func toggleFile(sel map[string]bool, p string) {
	if sel[p] {
		delete(sel, p)
	} else {
		sel[p] = true
	}
}

// selectAllVisible — a / A. В tree-режиме работает по всем файлам
// дерева, независимо от того, что свёрнуто. Так удобнее: «a» —
// «выбрать весь проект».
func (m *Model) selectAllVisible(on bool) {
	for _, e := range m.all {
		if on {
			m.selected[e.Path] = true
		} else {
			delete(m.selected, e.Path)
		}
	}
}

// toggleDirScope — d. В flat-режиме: родительская директория
// текущего файла. В tree-режиме: текущая директория, если курсор
// на директории; иначе — родитель текущего файла.
func (m *Model) toggleDirScope() {
	prefix := m.currentDirPrefix()
	if prefix == "" {
		return
	}
	// есть ли вообще файлы с этим префиксом
	any := false
	all := true
	for _, e := range m.all {
		if !strings.HasPrefix(e.Path, prefix) {
			continue
		}
		any = true
		if !m.selected[e.Path] {
			all = false
			break
		}
	}
	if !any {
		return
	}
	for _, e := range m.all {
		if strings.HasPrefix(e.Path, prefix) {
			if all {
				delete(m.selected, e.Path)
			} else {
				m.selected[e.Path] = true
			}
		}
	}
}

func (m Model) currentDirPrefix() string {
	if m.treeMode {
		it := m.currentTreeItem()
		if it == nil {
			return ""
		}
		if it.node.IsDir {
			return it.node.Path + "/"
		}
		if it.node.Parent != nil && it.node.Parent.Path != "" {
			return it.node.Parent.Path + "/"
		}
		return ""
	}
	e := m.currentFlatEntry()
	if e == nil {
		return ""
	}
	dir := path.Dir(e.Path)
	if dir == "." {
		return ""
	}
	return dir + "/"
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

// TreeMode сообщает, активен ли древовидный режим.
func (m Model) TreeMode() bool { return m.treeMode }

// ─── Flat-фильтр (fuzzy) ─────────────────────────────────────────────

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

// LimitCeiling возвращает эффективный потолок для индикатора лимита.
// 0 — лимит не задан, индикатор не показывается.
func (m Model) LimitCeiling() int {
	if m.opts.ContextLimit <= 0 {
		return 0
	}
	reserve := m.opts.Reserve
	if reserve <= 0 {
		reserve = tokens.DefaultReserve(m.opts.ContextLimit)
	}
	ceiling := m.opts.ContextLimit - reserve
	if m.opts.Budget > 0 && m.opts.Budget < ceiling {
		ceiling = m.opts.Budget
	}
	return ceiling
}
