package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	cursorStyle   = lipgloss.NewStyle().Reverse(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	dimStyle      = lipgloss.NewStyle().Faint(true)
	filterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	helpStyle     = lipgloss.NewStyle().Faint(true)
)

// View — требование tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		// ещё не пришёл WindowSizeMsg
		return "loading…"
	}

	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("cake pick"))
	b.WriteString(dimStyle.Render(" · " + m.opts.Root))
	b.WriteString("\n\n")

	// Body
	b.WriteString(m.renderList())
	b.WriteString("\n")

	// Filter (если активен или непустой)
	if m.inFilter || m.filter != "" {
		b.WriteString(filterStyle.Render("/" + m.filter))
		if m.inFilter {
			b.WriteString("▎")
		}
		b.WriteString("\n")
	}

	// Status
	b.WriteString(m.renderStatus())
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render(
		"↑↓ · space · a/A · d · / filter · tab mode · f format · ⏎ export · q quit",
	))

	return b.String()
}

func (m Model) renderList() string {
	// Фиксированные строки: header (2), фильтр (≤1), статус (1),
	// help (1), пустые разделители (2). Берём с запасом.
	available := m.height - 7
	if available < 3 {
		available = 3
	}

	// Скользящее окно вокруг курсора.
	start := 0
	if m.cursor >= available {
		start = m.cursor - available + 1
	}
	end := start + available
	if end > len(m.visible) {
		end = len(m.visible)
	}

	if len(m.visible) == 0 {
		return dimStyle.Render("  (нет совпадений)")
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		e := m.visible[i]
		mark := "[ ] "
		if m.selected[e.Path] {
			mark = "[x] "
		}
		line := mark + e.Path

		switch {
		case i == m.cursor:
			line = cursorStyle.Render(line)
		case m.selected[e.Path]:
			line = selectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	// Убираем последний \n, чтобы не было пустой строки в конце.
	s := b.String()
	return strings.TrimRight(s, "\n")
}

func (m Model) renderStatus() string {
	mode := "dump"
	if m.opts.Mode == pipeline.ModeClean {
		mode = "clean"
	}
	format := string(m.opts.Format)
	if format == "" {
		format = string(render.FormatXML)
	}

	return fmt.Sprintf("%s · %s · %d/%d selected · ~%d tok",
		mode, format,
		m.SelectedCount(), len(m.all),
		m.SelectedTokens(),
	)
}
