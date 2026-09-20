package tui

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/pkg/types"
)

// ─── Палитра ─────────────────────────────────────────────────────────
//
// Catppuccin-подобная схема: Latte для светлых терминалов,
// Mocha для тёмных. AdaptiveColor сам выбирает вариант по фону,
// так что TUI выглядит уместно и в светлой, и в тёмной теме.
var (
	cText    = lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"}
	cMuted   = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#6c7086"}
	cGuide   = lipgloss.AdaptiveColor{Light: "#bcc0cc", Dark: "#45475a"}
	cOverlay = lipgloss.AdaptiveColor{Light: "#dce0e8", Dark: "#313244"}
	cBlue    = lipgloss.AdaptiveColor{Light: "#1e66f5", Dark: "#89b4fa"}
	cGreen   = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"}
	cYellow  = lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"}
	cMauve   = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"}
	cRed     = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"}
)

// ─── Стили ───────────────────────────────────────────────────────────

var (
	titleStyle    = lipgloss.NewStyle().Foreground(cMauve).Bold(true)
	subtitleStyle = lipgloss.NewStyle().Foreground(cMuted)

	// Курсор — фоновая подсветка, а не Reverse:
	// Reverse инвертирует цвета контента и «звенит» на пестром
	// дереве; мягкий фон читается спокойнее и не спорит
	// с цветами selected/dir.
	cursorStyle = lipgloss.NewStyle().
			Background(cOverlay).
			Foreground(cText)

	selectedStyle = lipgloss.NewStyle().Foreground(cGreen)
	partialStyle  = lipgloss.NewStyle().Foreground(cYellow)
	dirStyle      = lipgloss.NewStyle().Foreground(cBlue).Bold(true)
	guideStyle    = lipgloss.NewStyle().Foreground(cGuide)
	dimStyle      = lipgloss.NewStyle().Foreground(cMuted)
	filterStyle   = lipgloss.NewStyle().Foreground(cYellow)
	helpKeyStyle  = lipgloss.NewStyle().Foreground(cMauve).Bold(true)

	// Бейджи режима — плашка с фоном. Без padding: он раздувает
	// ширину и ломает выравнивание статусной строки.
	badgeDumpStyle = lipgloss.NewStyle().
			Background(cBlue).Foreground(cOverlay).Bold(true)
	badgeCleanStyle = lipgloss.NewStyle().
			Background(cGreen).Foreground(cOverlay).Bold(true)
)

const (
	viewChrome = 5 // header(2) + пустая после списка(1) + статус(1) + help(1)
	filterLine = 1 // строка фильтра, зарезервирована всегда
)

// iconsEnabled — Nerd Font иконки включаются переменной окружения.
// По умолчанию выключены: не у всех стоит шрифт, а квадратики
// в UI выглядят хуже, чем ничего.
var iconsEnabled = os.Getenv("CAKE_ICONS") != ""

// maxContentWidth — максимальная ширина контентной области.
// На широких терминалах композиция центрируется, а не растягивается
// на всю ширину: длинные пути тяжело читать, когда глаз бегает
// от левого края к правому. Переопределяется CAKE_WIDTH=<n>.
var maxContentWidth = func() int {
	if v := os.Getenv("CAKE_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 40 {
			return n
		}
	}
	return 110
}()

// View сохраняет прежний вертикальный ритм (та же высота),
// меняется только оформление + горизонтальное центрирование
// композиции.
func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}

	width := m.contentWidth()

	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(fit(m.renderPath(), width)))
	b.WriteString("\n\n")

	b.WriteString(m.renderList(width))
	b.WriteString("\n")

	b.WriteString(m.renderFilterLine())
	b.WriteString("\n")

	b.WriteString(m.renderStatus())
	b.WriteString("\n")

	b.WriteString(m.renderHelp())

	// 1. Прижимаем каждую строку блока к левому краю внутри
	//    фиксированной ширины. Это выравнивает общий левый край:
	//    гайды дерева начинаются с одной колонки.
	//
	//    Без этого шага lipgloss.Place с Align(Center) центрирует
	//    каждую строку по её собственной длине — и ветки дерева
	//    «плывут»: чем глубже узел, тем левее он уезжает.
	block := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Left).
		Render(b.String())

	// 2. Теперь можно центрировать: все строки одной ширины,
	//    Place сдвинет их одинаково.
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Top,
		block,
	)
}

// contentWidth — ширина контентной области: вся доступная ширина
// на узких терминалах, maxContentWidth на широких.
func (m Model) contentWidth() int {
	w := m.width
	if w > maxContentWidth {
		w = maxContentWidth
	}
	if w < 20 {
		w = 20
	}
	return w
}

// ─── Шапка ───────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	return titleStyle.Render("cake")
}

// renderPath показывает корень с сокращением домашней папки.
// ~ вместо /Users/you — так короче и привычнее.
func (m Model) renderPath() string {
	root := m.opts.Root
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if root == home {
			root = "~"
		} else if strings.HasPrefix(root, home+string(os.PathSeparator)) {
			root = "~" + root[len(home):]
		}
	}
	return root
}

// ─── Список ──────────────────────────────────────────────────────────

func (m Model) renderList(width int) string {
	height := m.listHeight()
	if height < 1 {
		height = 1
	}

	total := m.currentLen()

	// offset поддерживает ensureCursorVisible (см. model.go).
	// Здесь только страховка от рассинхронизации после фильтра
	// или ресайза.
	maxOffset := total - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	start := m.offset
	if start > maxOffset {
		start = maxOffset
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > total {
		end = total
	}

	lines := make([]string, 0, height)

	if total == 0 {
		lines = append(lines, dimStyle.Render("  нет совпадений"))
	} else if m.treeMode {
		for i := start; i < end; i++ {
			lines = append(lines, m.renderTreeLine(m.flat[i], i == m.cursor, width))
		}
	} else {
		for i := start; i < end; i++ {
			lines = append(lines, m.renderFlatLine(m.visible[i], i == m.cursor, width))
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFlatLine(e types.FileEntry, isCursor bool, width int) string {
	mark, markStyle := "[ ]", dimStyle
	if m.selected[e.Path] {
		mark, markStyle = "[x]", selectedStyle
	}
	label := e.Path
	if icon := iconFor(e.Path, false); icon != "" {
		label = icon + label
	}

	if isCursor {
		// Курсор рендерим «сырым» текстом, потом красим целиком
		// на всю ширину — фон должен тянуться до правого края
		// контентной области, а не обрываться за текстом.
		return cursorLine(mark+" "+label, width)
	}
	return fit(markStyle.Render(mark)+" "+label, width)
}

func (m Model) renderTreeLine(it flatItem, isCursor bool, width int) string {
	n := it.node

	mark, markStyle := "[ ]", dimStyle
	switch {
	case n.IsDir:
		sel, total := selectState(n, m.selected)
		switch {
		case total > 0 && sel == total:
			mark, markStyle = "[x]", selectedStyle
		case sel > 0:
			mark, markStyle = "[-]", partialStyle
		}
	default:
		if m.selected[n.Path] {
			mark, markStyle = "[x]", selectedStyle
		}
	}

	label := n.Name
	if n.IsDir {
		glyph := "▸ "
		if n.Expanded {
			glyph = "▾ "
		}
		label = glyph + label + "/"
	}
	if icon := iconFor(n.Name, n.IsDir); icon != "" {
		label = icon + label
	}

	if isCursor {
		// Стили гайдов и лейблов теряем осознанно: подсветка
		// курсора должна быть монолитной плашкой, а не набором
		// разноцветных пятен поверх фона.
		return cursorLine(it.prefix+mark+" "+label, width)
	}

	// Гайды (│ ├── └──) — самый бледный слой: они структурируют
	// дерево, но не должны конкурировать с именами файлов.
	// Раньше они шли «как есть», из-за чего дерево выглядело
	// тяжелее, чем нужно.
	guide := guideStyle.Render(it.prefix)

	labelStyle := lipgloss.NewStyle()
	switch {
	case n.IsDir:
		labelStyle = dirStyle
	case strings.HasPrefix(mark, "[x]"):
		labelStyle = selectedStyle
	}
	return fit(guide+markStyle.Render(mark)+" "+labelStyle.Render(label), width)
}

// cursorLine рендерит строку с фоновой подсветкой на всю ширину
// контентной области. Сначала обрезаем до width (ANSI-safe),
// затем добиваем пробелами, чтобы фон дотянулся до правого края.
func cursorLine(s string, width int) string {
	s = lipgloss.NewStyle().MaxWidth(width).Render(s)
	pad := width - lipgloss.Width(s)
	if pad < 0 {
		pad = 0
	}
	return cursorStyle.Render(s + strings.Repeat(" ", pad))
}

// fit обрезает строку до width, если она длиннее. ANSI-safe:
// escape-последовательности стилей не учитываются при подсчёте
// ширины и не рвутся посередине.
func fit(s string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// ─── Строка фильтра ──────────────────────────────────────────────────

func (m Model) renderFilterLine() string {
	if !m.inFilter && m.filter == "" {
		return ""
	}
	s := filterStyle.Render("/") + m.filter
	if m.inFilter {
		s += filterStyle.Render("▎")
	}
	return s
}

// ─── Статус ──────────────────────────────────────────────────────────

func (m Model) renderStatus() string {
	var badge string
	if m.opts.Mode == pipeline.ModeClean {
		badge = badgeCleanStyle.Render(" clean ")
	} else {
		badge = badgeDumpStyle.Render(" dump ")
	}

	view := "list"
	if m.treeMode {
		view = "tree"
	}
	format := string(m.opts.Format)
	if format == "" {
		format = string(render.FormatXML)
	}

	pos := "0/0"
	if n := m.currentLen(); n > 0 {
		pos = fmt.Sprintf("%d/%d", m.cursor+1, n)
	}

	sep := dimStyle.Render(" · ")
	parts := []string{
		view,
		format,
		dimStyle.Render(pos),
		selectedStyle.Render(fmt.Sprintf("%d/%d sel", m.SelectedCount(), len(m.all))),
	}

	// Индикатор лимита заменяет обычную оценку токенов, если лимит
	// задан. Высота строки не меняется — новые элементы просто
	// дописываются в тот же join.
	if ind := m.limitIndicator(); ind != "" {
		parts = append(parts, ind)
	} else {
		parts = append(parts, selectedStyle.Render("~"+humanTokens(m.SelectedTokens())))
	}

	return badge + " " + strings.Join(parts, sep)
}

// limitIndicator возвращает строку вида "≈180k / 180k" с цветом по
// порогам 90% и 100%. Пустая строка, если лимит не задан.
//
// В clean-режиме оценка SelectedTokens идёт по Size (до
// процессоров), то есть завышена — рядом с числом ставится "≤".
// Пометка говорит: «не больше, точное значение после обработки».
func (m Model) limitIndicator() string {
	ceiling := m.LimitCeiling()
	if ceiling <= 0 {
		return ""
	}
	tok := m.SelectedTokens()
	pct := tok * 100 / ceiling

	label := fmt.Sprintf("≈%s / %s",
		humanTokensCompact(tok),
		humanTokensCompact(ceiling),
	)
	if m.opts.Mode == pipeline.ModeClean {
		label = "≤" + label
	}

	style := lipgloss.NewStyle().Foreground(cText)
	switch {
	case pct > 100:
		style = lipgloss.NewStyle().Foreground(cRed)
	case pct >= 90:
		style = lipgloss.NewStyle().Foreground(cYellow)
	}
	return style.Render(label)
}

// ─── Подсказка ───────────────────────────────────────────────────────

func (m Model) renderHelp() string {
	keys := []string{"↑↓", "space", "a/A", "d", "/", "tab", "f", "t", "⏎", "q"}
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(dimStyle.Render(" · "))
		}
		b.WriteString(helpKeyStyle.Render(k))
	}
	return b.String()
}

// ─── Вспомогательное ─────────────────────────────────────────────────

func (m Model) listHeight() int {
	h := m.height - viewChrome - filterLine
	if h < 3 {
		h = 3
	}
	return h
}

func humanTokens(n int) string {
	if n >= 10000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d tok", n)
}

// humanTokensCompact — для индикатора лимита: без «tok» и «~»,
// чтобы строка не распухала. 171000 → 171k, 3200 → 3.2k, 999 → 999.
func humanTokensCompact(n int) string {
	if n >= 10000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	if n >= 1000 {
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// ─── Иконки (Nerd Font, опционально) ─────────────────────────────────
//
// Включаются через CAKE_ICONS=1. Без флага функции возвращают "",
// layout не меняется — иконки не ломают тесты высоты, потому что
// добавляются внутри уже существующих строк.

func iconFor(name string, isDir bool) string {
	if !iconsEnabled {
		return ""
	}
	if isDir {
		return "󰉋 " // nf-md-folder
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".go":
		return "󰟓 " // nf-md-language_go
	case ".md":
		return "󰍔 " // nf-md-language_markdown
	case ".json":
		return "󰘦 "
	case ".yaml", ".yml":
		return "󰈙 "
	case ".toml":
		return "󰈙 "
	case ".sh", ".bash":
		return "󱆃 "
	case ".py":
		return "󰌠 "
	case ".rs":
		return "󱘗 "
	case ".js", ".ts", ".tsx", ".jsx":
		return "󰌞 "
	}
	return "󰈔 " // nf-md-file
}
