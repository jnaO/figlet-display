package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jnaO/figlet-font-preview/internal/catalog"
	"github.com/jnaO/figlet-font-preview/internal/fontclass"
)

func TestRevealOffset(t *testing.T) {
	const h = 10
	cases := []struct {
		name             string
		y, start, end, w int
	}{
		{"fully visible stays", 0, 2, 8, 0},
		{"exactly fits at bottom stays", 0, 4, 10, 0},
		{"below scrolls just enough", 0, 8, 14, 4},
		{"above scrolls to its top", 20, 12, 18, 12},
		{"taller than view shows top", 0, 5, 30, 5},
		{"taller than view above shows top", 40, 5, 30, 5},
	}
	for _, c := range cases {
		if got := revealOffset(c.y, h, c.start, c.end); got != c.w {
			t.Errorf("%s: got %d, want %d", c.name, got, c.w)
		}
	}
}

func TestLolcatPadsAndColours(t *testing.T) {
	out := lolcat([]string{"ab", ""}, 5)
	if len(out) != 2 || out[0] == "ab" {
		t.Fatalf("got %q", out)
	}
	if rainbow(lolSeed) == rainbow(lolSeed+1) {
		t.Fatal("adjacent rows should differ in colour")
	}
}

func testModel(t *testing.T, n int) Model {
	t.Helper()
	data, err := os.ReadFile("../figfont/testdata/tiny.flf")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for i := range n {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.flf", i)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := New(catalog.Discover([]string{dir}), "ab", false)
	<-m.loaded
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	return next.(Model)
}

func key(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "ctrl+o":
			msg = tea.KeyMsg{Type: tea.KeyCtrlO}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
		m.View() // components update scroll state from their last render
	}
	return m
}

func screenRow(m Model, name string) int { return m.spans[name].start - m.preview.YOffset }

func TestCursorFontKeepsRowWhenTextChanges(t *testing.T) {
	m := testModel(t, 30)
	m = key(m, "tab", "tab")
	for range 12 {
		m = key(m, "down")
	}
	name := m.cursorName()
	before := screenRow(m, name)
	if m.preview.YOffset == 0 {
		t.Fatal("expected the preview to have scrolled")
	}
	m = key(m, "tab", "c", "d", "enter", "e", "f") // list → text; add a second line
	if got := screenRow(m, name); got != before {
		t.Fatalf("cursor font moved from row %d to %d", before, got)
	}
	if m.text.Height() != 2 {
		t.Fatalf("text area height %d, want 2", m.text.Height())
	}
}

func withIndex(m Model) Model {
	next, _ := m.Update(indexMsg{fontclass.Build(m.cat)})
	return next.(Model)
}

func settle(m Model) Model {
	for range 10 {
		next, _ := m.Update(slideMsg{})
		m = next.(Model)
	}
	return m
}

func TestOptionsPaneFromAnyFocus(t *testing.T) {
	next, _ := testModel(t, 3).Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m := next.(Model)
	full := m.previewWidth()
	m = settle(key(m, "ctrl+o")) // from the text area
	if !m.panelOpen || m.focus != focusOptions || m.panelW != optionsWidth {
		t.Fatalf("open=%v focus=%v width=%d", m.panelOpen, m.focus, m.panelW)
	}
	if m.previewWidth() != full-optionsWidth-1 {
		t.Fatalf("preview width %d, want %d", m.previewWidth(), full-optionsWidth-1)
	}
	m = settle(key(m, "ctrl+o"))
	if m.panelOpen || m.focus != focusText || m.panelW != 0 {
		t.Fatalf("after close: open=%v focus=%v width=%d", m.panelOpen, m.focus, m.panelW)
	}
	m = settle(key(m, "tab", "ctrl+o", "esc")) // esc closes too, back to search
	if m.panelOpen || m.focus != focusSearch {
		t.Fatalf("esc: open=%v focus=%v", m.panelOpen, m.focus)
	}
	m = key(m, "ctrl+o", "tab") // tab leaves the pane open
	if !m.panelOpen || m.focus != focusText {
		t.Fatalf("tab: open=%v focus=%v", m.panelOpen, m.focus)
	}
}

func TestOptionsCenterToggle(t *testing.T) {
	m := key(testModel(t, 3), "ctrl+o", "space") // first row is Center
	if !m.state.Center {
		t.Fatal("Center should be on")
	}
}

func TestOptionsInertBeforeIndex(t *testing.T) {
	m := key(testModel(t, 3), "ctrl+o")
	for range 4 { // past Collapse, Group by and Similar to the first facet row
		m = key(m, "down")
	}
	m = key(m, "space")
	if m.state.HasFilters() || m.state.Collapse {
		t.Fatal("options must do nothing before the index arrives")
	}
}

func TestOptionsSimilarAnchorsCursorFont(t *testing.T) {
	m := withIndex(testModel(t, 5))
	m = key(m, "tab", "tab", "down", "down") // cursor on f02
	m = key(m, "ctrl+o", "down", "down", "down", "space")
	if m.state.Similar() != "f02" {
		t.Fatalf("similar = %q, want f02", m.state.Similar())
	}
	if m.items[0].Name != "f02" {
		t.Fatalf("anchor should lead the sidebar, got %q", m.items[0].Name)
	}
	if m = key(m, "space"); m.state.Similar() != "" {
		t.Fatal("toggling again should clear the anchor")
	}
}

func TestGroupedSidebarHeaders(t *testing.T) {
	m := withIndex(testModel(t, 4))
	m = key(m, "tab", "tab", "ctrl+o", "down", "down", "right", "esc") // Group by → Stroke
	if m.state.GroupBy != fontclass.Stroke || !m.items[0].IsHeader() {
		t.Fatalf("expected a Stroke header first, got %+v", m.items[0])
	}
	m.cursor = 0
	y := m.preview.YOffset
	m = key(m, "space")
	if m.state.SelectedCount() != 4 {
		t.Fatalf("header should select its group, selected %d", m.state.SelectedCount())
	}
	if m.preview.YOffset != y || m.cursorName() != "" {
		t.Fatal("a header has no font and must not scroll the preview")
	}
	if m = key(m, "space"); m.state.SelectedCount() != 0 {
		t.Fatal("header should deselect a fully selected group")
	}
}
