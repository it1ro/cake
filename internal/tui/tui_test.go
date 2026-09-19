package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/it1ro/cake/internal/pipeline"
	"github.com/it1ro/cake/internal/render"
	"github.com/it1ro/cake/pkg/types"
)

func newModel(entries ...string) Model {
	es := make([]types.FileEntry, len(entries))
	for i, p := range entries {
		es[i] = types.FileEntry{Path: p, Size: 100}
	}
	return New(es, pipeline.Options{Format: render.FormatXML})
}

// key посылает клавишу в модель и возвращает обновлённую копию.
//
// Для печатаемых символов используем KeyRunes — bubbletea
// возвращает их String() как исходную строку. Для спецклавиш
// собираем KeyMsg с конкретным Type.
func key(m Model, s string) Model {
	var msg tea.KeyMsg
	switch s {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestSelectToggle(t *testing.T) {
	m := newModel("a.go", "b.go", "c.go")

	m = key(m, " ")
	if !m.selected["a.go"] {
		t.Error("space должен выбрать a.go")
	}
	m = key(m, " ")
	if m.selected["a.go"] {
		t.Error("повторный space должен снять выбор")
	}
}

func TestCursorMovement(t *testing.T) {
	m := newModel("a.go", "b.go", "c.go")
	m = key(m, "down")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m = key(m, "up")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	// up на верхней границе не уходит в минус
	m = key(m, "up")
	if m.cursor != 0 {
		t.Errorf("cursor должен остаться 0, got %d", m.cursor)
	}
}

func TestSelectAll(t *testing.T) {
	m := newModel("a.go", "b.go", "c.go")
	m = key(m, "a")
	if m.SelectedCount() != 3 {
		t.Errorf("a должен выбрать всё, got %d", m.SelectedCount())
	}
	m = key(m, "A")
	if m.SelectedCount() != 0 {
		t.Errorf("A должен снять всё, got %d", m.SelectedCount())
	}
}

func TestDirectoryToggle(t *testing.T) {
	m := newModel("cmd/a.go", "cmd/b.go", "internal/x.go")
	// курсор на cmd/a.go
	m = key(m, "d")
	if !m.selected["cmd/a.go"] || !m.selected["cmd/b.go"] {
		t.Error("d должен выбрать всё в cmd/")
	}
	if m.selected["internal/x.go"] {
		t.Error("d не должен затронуть internal/")
	}
	// повторный d — снимает
	m = key(m, "d")
	if m.selected["cmd/a.go"] || m.selected["cmd/b.go"] {
		t.Error("повторный d должен снять выбор в cmd/")
	}
}

func TestFilter(t *testing.T) {
	m := newModel("cmd/cake/main.go", "internal/walker/walker.go", "README.md")
	m = key(m, "/")
	if !m.inFilter {
		t.Fatal("/ должен включить режим фильтра")
	}
	for _, c := range "wal" {
		m = key(m, string(c))
	}
	if len(m.visible) == 0 {
		t.Fatal("фильтр walker должен что-то найти")
	}
	for _, e := range m.visible {
		if e.Path == "README.md" {
			t.Errorf("README.md не должен пройти фильтр walker: %v", m.visible)
		}
	}
	m = key(m, "esc")
	if m.filter != "" || m.inFilter {
		t.Error("esc должен сбросить фильтр и выйти из режима")
	}
	if len(m.visible) != len(m.all) {
		t.Error("после esc должен быть виден весь список")
	}
}

func TestModeAndFormatCycle(t *testing.T) {
	m := newModel("a.go")
	if m.opts.Mode != pipeline.ModeDump {
		t.Fatal("стартовый режим — dump")
	}
	m = key(m, "tab")
	if m.opts.Mode != pipeline.ModeClean {
		t.Error("tab → clean")
	}
	m = key(m, "tab")
	if m.opts.Mode != pipeline.ModeDump {
		t.Error("tab → dump")
	}

	if m.opts.Format != render.FormatXML {
		t.Fatal("стартовый формат — xml")
	}
	m = key(m, "f")
	if m.opts.Format != render.FormatMarkdown {
		t.Error("f: xml → markdown")
	}
	m = key(m, "f")
	if m.opts.Format != render.FormatPlain {
		t.Error("f: markdown → plain")
	}
	m = key(m, "f")
	if m.opts.Format != render.FormatXML {
		t.Error("f: plain → xml")
	}
}

func TestConfirmed(t *testing.T) {
	m := newModel("a.go", "b.go")
	m = key(m, " ")
	m = key(m, "enter")
	if !m.Confirmed() {
		t.Error("enter должен выставить Confirmed")
	}
	sel := m.SelectedFiles()
	if len(sel) != 1 || sel[0].Path != "a.go" {
		t.Errorf("SelectedFiles = %+v, want [a.go]", sel)
	}
}

// Порядок вывода — по all (отсортированному Plan'ом), а не по
// порядку кликов. Иначе детерминизм рендера теряется.
func TestSelectedFiles_PreservesOrder(t *testing.T) {
	m := newModel("a.go", "b.go", "c.go")
	m = key(m, "G") // курсор на c.go
	m = key(m, " ")
	m = key(m, "g") // курсор на a.go
	m = key(m, " ")
	sel := m.SelectedFiles()
	if len(sel) != 2 || sel[0].Path != "a.go" || sel[1].Path != "c.go" {
		t.Errorf("SelectedFiles = %+v, want [a.go c.go]", sel)
	}
}

func TestSelectedTokens(t *testing.T) {
	m := newModel("a.go", "b.go") // Size=100 каждый → 25 tok
	m = key(m, " ")
	if got := m.SelectedTokens(); got != 25 {
		t.Errorf("SelectedTokens = %d, want 25", got)
	}
}

// Без WindowSizeMsg View() не должен паниковать.
func TestViewBeforeResize(t *testing.T) {
	m := newModel("a.go")
	if got := m.View(); got == "" {
		t.Error("View до resize должен что-то возвращать, не пустоту")
	}
}
