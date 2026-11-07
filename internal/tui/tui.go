// Package tui is the Bubble Tea front-end over internal/display.
package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jnaO/figlet-display/internal/catalog"
	"github.com/jnaO/figlet-display/internal/display"
)

const (
	sidebarWidth = 30 // content columns, excluding the divider
	tickEvery    = 100 * time.Millisecond
)

type focus int

const (
	focusText focus = iota
	focusSearch
	focusList
	focusCount
)

type tickMsg struct{}

// span is a half-open range of preview lines.
type span struct{ start, end int }

// Model is the root Bubble Tea model.
type Model struct {
	cat     *catalog.Catalog
	state   *display.State
	loaded  <-chan struct{}
	loading bool

	text, search textinput.Model
	preview      viewport.Model
	focus        focus

	items          []display.Item
	cursor, offset int
	spans          map[string]span // preview line range of each rendered font

	width, height int
	ready         bool
}

// New builds the model and starts parsing every font in the background.
func New(cat *catalog.Catalog, text string) Model {
	ti := textinput.New()
	ti.Prompt = "Text › "
	ti.PromptStyle = styles.label
	ti.SetValue(text)
	ti.Focus()

	si := textinput.New()
	si.Prompt = "/ "
	si.PromptStyle = styles.label
	si.Placeholder = "fuzzy search fonts"

	m := Model{
		cat:     cat,
		state:   display.New(cat.Names(), text),
		loaded:  cat.Preload(runtime.NumCPU()),
		loading: true,
		text:    ti,
		search:  si,
		preview: viewport.New(0, 0),
	}
	m.items = m.state.Sidebar()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tick())
}

func tick() tea.Cmd {
	return tea.Tick(tickEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height, m.ready = msg.Width, msg.Height, true
		m.layout()
		m.refreshPreview(false)
		return m, nil

	case tickMsg:
		select {
		case <-m.loaded:
			m.loading = false
		default:
		}
		m.refreshPreview(false)
		if m.loading {
			return m, tick()
		}
		return m, nil

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	switch m.focus {
	case focusText:
		m.text, cmd = m.text.Update(msg)
	case focusSearch:
		m.search, cmd = m.search.Update(msg)
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		return m, m.setFocus((m.focus + 1) % focusCount)
	case "shift+tab":
		return m, m.setFocus((m.focus + focusCount - 1) % focusCount)
	case "pgup", "pgdown", "ctrl+u", "ctrl+d":
		m.scrollPreview(msg.String())
		return m, nil
	}

	switch m.focus {
	case focusText:
		var cmd tea.Cmd
		m.text, cmd = m.text.Update(msg)
		if v := m.text.Value(); v != m.state.Text {
			m.state.Text = v
			m.refreshPreview(false)
		}
		return m, cmd

	case focusSearch:
		switch msg.String() {
		case "esc":
			m.search.SetValue("")
		case "up", "down":
			m.moveCursor(msg.String())
			return m, nil
		case "enter":
			m.toggleCursor()
			return m, nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.applyQuery()
		return m, cmd

	case focusList:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "/":
			return m, m.setFocus(focusSearch)
		case "esc":
			m.search.SetValue("")
			m.applyQuery()
		case "up", "k", "down", "j", "home", "g", "end", "G":
			m.moveCursor(msg.String())
		case " ", "enter":
			m.toggleCursor()
		case "x":
			m.state.ClearSelection()
			m.refreshPreview(true)
		}
	}
	return m, nil
}

func (m *Model) setFocus(f focus) tea.Cmd {
	m.focus = f
	m.text.Blur()
	m.search.Blur()
	switch f {
	case focusText:
		return m.text.Focus()
	case focusSearch:
		return m.search.Focus()
	}
	return nil
}

func (m *Model) applyQuery() {
	if q := m.search.Value(); q != m.state.Query() {
		m.state.SetQuery(q)
		m.items = m.state.Sidebar()
		m.cursor, m.offset = 0, 0
		m.refreshPreview(true)
		m.revealCursor()
	}
}

func (m *Model) moveCursor(key string) {
	switch key {
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.items) - 1
	}
	m.cursor = max(0, min(m.cursor, len(m.items)-1))
	m.clampOffset()
	m.refreshPreview(false)
	m.revealCursor()
}

func (m Model) cursorName() string {
	if m.cursor < len(m.items) {
		return m.items[m.cursor].Name
	}
	return ""
}

// revealCursor scrolls the preview only as far as needed to show the cursor
// font in full; fonts outside the render set leave the scroll untouched.
func (m *Model) revealCursor() {
	if s, ok := m.spans[m.cursorName()]; ok {
		m.preview.SetYOffset(revealOffset(m.preview.YOffset, m.preview.Height, s.start, s.end))
	}
}

func (m *Model) toggleCursor() {
	if m.cursor < len(m.items) {
		m.state.Toggle(m.items[m.cursor].Name)
		m.refreshPreview(true)
		m.revealCursor()
	}
}

func (m *Model) scrollPreview(key string) {
	switch key {
	case "pgup":
		m.preview.ViewUp()
	case "pgdown":
		m.preview.ViewDown()
	case "ctrl+u":
		m.preview.HalfViewUp()
	case "ctrl+d":
		m.preview.HalfViewDown()
	}
}

// Layout: header row, body (sidebar | preview), footer row.
func (m *Model) layout() {
	bodyH := max(m.height-2, 1)
	m.text.Width = max(m.width-lipgloss.Width(m.text.Prompt)-1, 1)
	m.search.Width = sidebarWidth - lipgloss.Width(m.search.Prompt) - 1
	m.preview.Width = m.previewWidth()
	m.preview.Height = bodyH
	m.clampOffset()
}

func (m Model) previewWidth() int { return max(m.width-sidebarWidth-3, 1) }

// listHeight is the number of font rows the sidebar shows.
func (m Model) listHeight() int { return max(m.height-2-3, 1) }

func (m *Model) clampOffset() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(0, min(m.offset, max(len(m.items)-h, 0)))
}

// refreshPreview re-renders the preview; reset scrolls it to the top.
func (m *Model) refreshPreview(reset bool) {
	if !m.ready {
		return
	}
	content, spans := m.previewContent()
	m.spans = spans
	m.preview.SetContent(content)
	if reset {
		m.preview.GotoTop()
	}
}

// previewContent renders the render set and records each font's line span.
// The cursor font is drawn in rainbow colours on a tinted background.
func (m Model) previewContent() (string, map[string]span) {
	if len(m.cat.Entries) == 0 {
		return styles.dim.Render(fmt.Sprintf(
			"No fonts found.\n\nSearched:\n  %s\n\nAdd more with --font-dir <dir>.",
			strings.Join(m.cat.Roots, "\n  "))), nil
	}
	set := m.state.RenderSet()
	if len(set) == 0 {
		return styles.dim.Render("No fonts match the search."), nil
	}
	w := m.previewWidth()
	cursor := m.cursorName()
	spans := make(map[string]span, len(set))
	var lines []string
	for _, name := range set {
		hl := name == cursor
		start := len(lines)
		lines = append(lines, m.fontHeader(name, w, hl))
		e, _ := m.cat.Get(name)
		f, err := e.Font()
		var body []string
		switch {
		case err != nil:
			body = []string{truncate(err.Error(), w)}
		case f == nil:
			body = []string{"loading…"}
		default:
			body = f.Render(m.state.Text, w)
		}
		switch {
		case hl && f != nil:
			body = lolcat(body, w)
		case hl:
			for i, l := range body {
				body[i] = tintLine(l, w)
			}
		case err != nil:
			body = []string{styles.err.Render(body[0])}
		case f == nil:
			body = []string{styles.dim.Render(body[0])}
		}
		lines = append(lines, body...)
		spans[name] = span{start, len(lines)}
		lines = append(lines, "")
	}
	return strings.Join(lines[:len(lines)-1], "\n"), spans
}

func (m Model) fontHeader(name string, w int, hl bool) string {
	label := " " + name + " "
	style, rule := styles.header, styles.rule
	if m.state.IsSelected(name) {
		label = " ● " + name + " "
		style = styles.headerSelected
	}
	if hl {
		style, rule = style.Inherit(styles.tint), rule.Inherit(styles.tint)
	}
	n := max(w-lipgloss.Width(label)-2, 0)
	return style.Render("──"+label) + rule.Render(strings.Repeat("─", n))
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}
	header := m.text.View()
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.sidebarView(),
		styles.divider.Render(strings.Repeat("│\n", max(m.height-3, 0))+"│"),
		lipgloss.NewStyle().PaddingLeft(1).Render(m.preview.View()),
	)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, m.footerView())
}

func (m Model) sidebarView() string {
	lines := []string{
		m.search.View(),
		styles.dim.Render(m.countLine()),
		styles.rule.Render(strings.Repeat("─", sidebarWidth)),
	}
	end := min(m.offset+m.listHeight(), len(m.items))
	for i := m.offset; i < end; i++ {
		lines = append(lines, m.itemView(i))
	}
	return lipgloss.NewStyle().Width(sidebarWidth).Height(max(m.height-2, 1)).
		MaxHeight(max(m.height-2, 1)).Render(strings.Join(lines, "\n"))
}

func (m Model) countLine() string {
	s := fmt.Sprintf("%d/%d fonts · %d selected", len(m.items), len(m.cat.Entries), m.state.SelectedCount())
	if m.loading {
		s += " · loading"
	}
	return truncate(s, sidebarWidth)
}

func (m Model) itemView(i int) string {
	it := m.items[i]
	mark := styles.dim.Render("○ ")
	if m.state.IsSelected(it.Name) {
		mark = styles.selected.Render("● ")
	}
	name := highlight(truncate(it.Name, sidebarWidth-4), it.Matched)
	row := mark + name
	if i == m.cursor {
		cur := styles.cursor
		if m.focus == focusText {
			cur = styles.cursorInactive
		}
		return cur.Render("›") + " " + row
	}
	return "  " + row
}

func (m Model) footerView() string {
	var keys string
	switch m.focus {
	case focusText:
		keys = "type text · tab: search · pgup/pgdn: scroll · ctrl+c: quit"
	case focusSearch:
		keys = "type to filter · ↑/↓: move · enter: select · esc: clear · tab: list"
	case focusList:
		keys = "↑/↓ j/k: move · space: select · x: clear selection · /: search · tab: text · q: quit"
	}
	return styles.dim.Render(truncate(keys, m.width))
}

// highlight styles the runes of s at the matched indexes.
func highlight(s string, matched []int) string {
	if len(matched) == 0 {
		return s
	}
	hit := map[int]bool{}
	for _, i := range matched {
		hit[i] = true
	}
	var b strings.Builder
	for i, r := range s {
		if hit[i] {
			b.WriteString(styles.match.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:max(w, 0)])
	}
	return string(r[:w-1]) + "…"
}

// revealOffset returns the smallest change to the scroll offset y (viewport
// height h) that brings lines [start, end) fully into view, or the block's
// first line when it is taller than the viewport.
func revealOffset(y, h, start, end int) int {
	switch {
	case start < y:
		return start
	case end > y+h:
		return min(start, end-h)
	}
	return y
}
