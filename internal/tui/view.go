package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/pkg/types"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	cursorStyle   = lipgloss.NewStyle().Reverse(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	partialStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dirStyle      = lipgloss.NewStyle().Bold(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	filterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	helpStyle     = lipgloss.NewStyle().Faint(true)
)

const (
	viewChrome = 5 // header(2) + пустая после списка(1) + статус(1) + help(1)
	filterLine = 1 // строка фильтра, зарезервирована всегда
)

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("cake pick"))
	b.WriteString(dimStyle.Render(" · " + m.opts.Root))
	b.WriteString("\n\n")

	b.WriteString(m.renderList())
	b.WriteString("\n")

	if m.inFilter || m.filter != "" {
		b.WriteString(filterStyle.Render("/" + m.filter))
		if m.inFilter {
			b.WriteString("▎")
		}
	}
	b.WriteString("\n")

	b.WriteString(m.renderStatus())
	b.WriteString("\n")

	help := "↑↓ · space · a/A · d · / filter · tab mode · f format · t tree · ⏎ export · q quit"
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func (m Model) renderList() string {
	height := m.listHeight()
	if height < 1 {
		height = 1
	}

	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}
	end := start + height
	total := m.currentLen()
	if end > total {
		end = total
	}

	lines := make([]string, 0, height)

	if total == 0 {
		lines = append(lines, dimStyle.Render("  (нет совпадений)"))
	} else if m.treeMode {
		for i := start; i < end; i++ {
			lines = append(lines, m.renderTreeLine(m.flat[i], i == m.cursor))
		}
	} else {
		for i := start; i < end; i++ {
			lines = append(lines, m.renderFlatLine(m.visible[i], i == m.cursor))
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFlatLine(e types.FileEntry, isCursor bool) string {
	mark := "[ ] "
	if m.selected[e.Path] {
		mark = "[x] "
	}
	line := mark + e.Path

	switch {
	case isCursor:
		line = cursorStyle.Render(line)
	case m.selected[e.Path]:
		line = selectedStyle.Render(line)
	}
	return line
}

func (m Model) renderTreeLine(it flatItem, isCursor bool) string {
	n := it.node

	mark := "[ ] "
	switch {
	case n.IsDir:
		sel, total := selectState(n, m.selected)
		switch {
		case total > 0 && sel == total:
			mark = "[x] "
		case sel > 0:
			mark = "[-] "
		}
	default:
		if m.selected[n.Path] {
			mark = "[x] "
		}
	}

	name := n.Name
	if n.IsDir {
		name += "/"
		if n.Expanded {
			name = "▾ " + name
		} else {
			name = "▸ " + name
		}
	}

	plain := it.prefix + mark + name

	if isCursor {
		return cursorStyle.Render(plain)
	}

	// Стилизация: mark отдельным цветом, но с сохранением позиции.
	var style lipgloss.Style
	switch {
	case strings.HasPrefix(mark, "[x]"):
		style = selectedStyle
	case strings.HasPrefix(mark, "[-]"):
		style = partialStyle
	case n.IsDir:
		style = dirStyle
	default:
		return plain
	}
	return it.prefix + style.Render(mark+name)
}

func (m Model) listHeight() int {
	h := m.height - viewChrome - filterLine
	if h < 3 {
		h = 3
	}
	return h
}

func (m Model) renderStatus() string {
	mode := "dump"
	if m.opts.Mode == pipeline.ModeClean {
		mode = "clean"
	}
	view := "list"
	if m.treeMode {
		view = "tree"
	}
	format := string(m.opts.Format)
	if format == "" {
		format = string(render.FormatXML)
	}

	return fmt.Sprintf("%s · %s · %s · %d/%d selected · ~%d tok",
		mode, view, format,
		m.SelectedCount(), len(m.all),
		m.SelectedTokens(),
	)
}
