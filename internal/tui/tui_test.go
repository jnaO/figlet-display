package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jnaO/figlet-display/internal/catalog"
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
	m := New(catalog.Discover([]string{dir}), "ab")
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
