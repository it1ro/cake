package tui

import (
	"fmt"
	"strings"
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

// View должен быть стабильной высоты независимо от того,
// сколько файлов видно после фильтрации. Иначе нижний край
// (статус, help) пляшет при вводе фильтра.
func TestViewHeightStable(t *testing.T) {
	entries := make([]string, 30)
	for i := range entries {
		entries[i] = fmt.Sprintf("dir%02d/file.go", i)
	}
	m := newModel(entries...)
	m.width, m.height = 80, 24

	base := strings.Count(m.View(), "\n")

	// С фильтром, который оставляет 2 файла.
	m = key(m, "/")
	for _, c := range "dir00" {
		m = key(m, string(c))
	}
	filtered := strings.Count(m.View(), "\n")

	// С фильтром, который не находит ничего.
	m = key(m, "esc")
	m = key(m, "/")
	for _, c := range "zzzzzz" {
		m = key(m, string(c))
	}
	empty := strings.Count(m.View(), "\n")

	if base != filtered {
		t.Errorf("высота с фильтром (%d) != без фильтра (%d)", filtered, base)
	}
	if base != empty {
		t.Errorf("высота с пустым фильтром (%d) != без фильтра (%d)", empty, base)
	}
}

func TestTreeModeToggle(t *testing.T) {
	m := newModel("a/b.go", "a/c.go", "d.go")
	if m.TreeMode() {
		t.Fatal("стартовый режим — flat")
	}
	m = key(m, "t")
	if !m.TreeMode() {
		t.Error("t должен включить tree")
	}
	m = key(m, "t")
	if m.TreeMode() {
		t.Error("повторный t — обратно в flat")
	}
}

func TestTreeCollapse(t *testing.T) {
	m := newModel("cmd/main.go", "cmd/helper.go", "README.md")
	m = key(m, "t")
	m = key(m, "g") // t сохраняет курсор на текущем файле — сбрасываем на верх

	it := m.currentTreeItem()
	if it == nil || it.node.Name != "cmd" {
		t.Fatalf("ожидался курсор на cmd, got %+v", it)
	}
	before := len(m.flat)
	m = key(m, "left")
	after := len(m.flat)
	if after >= before {
		t.Errorf("collapse должен уменьшить список: %d → %d", before, after)
	}
	m = key(m, "right")
	if len(m.flat) != before {
		t.Errorf("expand должен вернуть: %d → %d", after, len(m.flat))
	}
}

func TestTreeSelectDir(t *testing.T) {
	m := newModel("cmd/main.go", "cmd/helper.go", "README.md")
	m = key(m, "t")
	m = key(m, "g")

	m = key(m, " ")
	if !m.selected["cmd/main.go"] || !m.selected["cmd/helper.go"] {
		t.Error("space на cmd должен выбрать все файлы под ней")
	}
	if m.selected["README.md"] {
		t.Error("space на cmd не должен трогать README.md")
	}

	m = key(m, " ")
	if m.selected["cmd/main.go"] || m.selected["cmd/helper.go"] {
		t.Error("повторный space должен снять выбор под cmd")
	}
}

func TestTreePartialSelect(t *testing.T) {
	m := newModel("cmd/a.go", "cmd/b.go")
	m = key(m, "t")
	m = key(m, "g")

	// cmd/ уже раскрыт по умолчанию; right — no-op, но пусть будет
	m = key(m, "right")
	m = key(m, "down")
	m = key(m, " ")

	m = key(m, "up")
	it := m.currentTreeItem()
	if it == nil || !it.node.IsDir {
		t.Fatal("курсор должен быть на cmd")
	}
	sel, total := selectState(it.node, m.selected)
	if sel != 1 || total != 2 {
		t.Errorf("selectState = (%d, %d), want (1, 2)", sel, total)
	}
}

func TestTreeFilterKeepsParents(t *testing.T) {
	m := newModel("cmd/cake/main.go", "internal/walker/walker.go", "README.md")
	m = key(m, "t")
	m = key(m, "/")
	for _, c := range "walker" {
		m = key(m, string(c))
	}
	// В дереве должно остаться: internal/ (родитель), walker/ (родитель),
	// walker.go (совпадение). README.md и cmd/ исчезли.
	var names []string
	for _, it := range m.flat {
		names = append(names, it.node.Path)
	}
	hasWalker := false
	hasCmd := false
	for _, n := range names {
		if strings.HasPrefix(n, "internal/walker") {
			hasWalker = true
		}
		if strings.HasPrefix(n, "cmd") {
			hasCmd = true
		}
	}
	if !hasWalker {
		t.Errorf("walker не найден: %v", names)
	}
	if hasCmd {
		t.Errorf("cmd не должен остаться: %v", names)
	}
}

// t должен сохранять курсор на текущем файле — иначе пользователь,
// листающий список и решивший глянуть дерево, теряет контекст.
func TestTreeModePreservesCursor(t *testing.T) {
	m := newModel("cmd/main.go", "cmd/helper.go", "README.md")
	// в flat cursor=0 → cmd/main.go
	m = key(m, "t")
	it := m.currentTreeItem()
	if it == nil {
		t.Fatal("ожидался узел после toggle")
	}
	if it.node.Path != "cmd/main.go" {
		t.Errorf("курсор должен остаться на cmd/main.go, got %q", it.node.Path)
	}

	// обратно — тот же файл
	m = key(m, "t")
	if e := m.currentFlatEntry(); e == nil || e.Path != "cmd/main.go" {
		t.Errorf("курсор должен вернуться на cmd/main.go, got %+v", e)
	}
}
