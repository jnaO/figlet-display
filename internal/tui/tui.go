// Package tui is the Bubble Tea front-end over internal/display.
package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jnaO/figlet-font-preview/internal/catalog"
	"github.com/jnaO/figlet-font-preview/internal/display"
	"github.com/jnaO/figlet-font-preview/internal/figfont"
)

const (
	sidebarWidth = 30 // content columns, excluding the divider
	maxTextLines = 5  // the text area grows up to this many rows, then scrolls
	textPrompt   = "Text › "
	tickEvery    = 100 * time.Millisecond
)

type focus int

const (
	focusText focus = iota
	focusSearch
	focusList
	focusOptions // only while the options pane is open
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

	text    textarea.Model
	search  textinput.Model
	preview viewport.Model
	focus   focus

	items          []display.Item
	cursor, offset int
	spans          map[string]span // preview line range of each rendered font

	panelOpen            bool  // the options pane is open (or sliding in)
	panelW               int   // the pane's current width while sliding
	prevFocus            focus // focus to return to when the pane closes
	optCursor, optOffset int

	width, height int
	ready         bool
}

// New builds the model and starts parsing every font in the background.
func New(cat *catalog.Catalog, text string, center bool) Model {
	ti := textarea.New()
	ti.ShowLineNumbers = false
	ti.EndOfBufferCharacter = ' '
	ti.CharLimit = 0
	ti.SetPromptFunc(lipgloss.Width(textPrompt), func(line int) string {
		if line == 0 {
			return textPrompt
		}
		return strings.Repeat(" ", lipgloss.Width(textPrompt)-2) + "┆ "
	})
	for _, st := range []*textarea.Style{&ti.FocusedStyle, &ti.BlurredStyle} {
		st.CursorLine = lipgloss.NewStyle()
		st.Prompt = styles.label
		st.Text = lipgloss.NewStyle()
	}
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
	m.state.Center = center
	m.items = m.state.Sidebar()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tick())
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
		return m, buildIndex(m.cat)

	case slideMsg:
		return m, m.stepSlide()

	case indexMsg:
		m.state.SetIndex(msg.index)
		m.itemsChanged()
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
	case "ctrl+o":
		return m, m.toggleOptions()
	case "tab":
		return m, m.setFocus(m.nextFocus(1))
	case "shift+tab":
		return m, m.setFocus(m.nextFocus(-1))
	case "pgup", "pgdown", "ctrl+u", "ctrl+d":
		m.scrollPreview(msg.String())
		return m, nil
	}

	switch m.focus {
	case focusText:
		var cmd tea.Cmd
		// The textarea scrolls its own view during Update based on its
		// current height, so it updates at full height and is then fitted.
		prevH, prevLines := m.text.Height(), m.text.LineCount()
		m.text.SetHeight(maxTextLines)
		m.text, cmd = m.text.Update(msg)
		if n := m.text.LineCount(); n < prevLines && n <= maxTextLines {
			m.resetTextScroll()
		}
		if m.textHeight() != prevH {
			m.layout()
		} else {
			m.text.SetHeight(prevH)
		}
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

	case focusOptions:
		return m.handlePanelKey(msg.String())

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

// nextFocus cycles text → search → list (→ options while open).
func (m Model) nextFocus(d int) focus {
	n := int(focusList) + 1
	if m.panelOpen {
		n++
	}
	return focus((int(m.focus) + d + n) % n)
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
	if m.cursor >= len(m.items) {
		return
	}
	if it := m.items[m.cursor]; it.IsHeader() {
		m.state.ToggleGroup(it.Header)
	} else {
		m.state.Toggle(it.Name)
	}
	m.refreshPreview(true)
	m.revealCursor()
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

// Layout: text area (1..maxTextLines rows), body (sidebar | preview), footer.
func (m *Model) layout() {
	m.text.SetWidth(max(m.width-1, 1))
	m.text.SetHeight(m.textHeight())
	m.search.Width = sidebarWidth - lipgloss.Width(m.search.Prompt) - 1
	m.preview.Width = m.previewWidth()
	m.preview.Height = m.bodyHeight()
	m.clampOffset()
}

// resetTextScroll scrolls the textarea back to its first line, keeping the
// cursor: textarea keeps a stale scroll offset when lines are deleted, and
// SetValue is the only way to reset it.
func (m *Model) resetTextScroll() {
	row := m.text.Line()
	info := m.text.LineInfo()
	col := info.StartColumn + info.ColumnOffset
	m.text.SetValue(m.text.Value())
	for m.text.Line() > row {
		m.text.CursorUp()
	}
	m.text.SetCursor(col)
}

func (m Model) textHeight() int { return max(1, min(m.text.LineCount(), maxTextLines)) }

func (m Model) bodyHeight() int { return max(m.height-m.textHeight()-1, 1) }

func (m Model) previewWidth() int {
	w := m.width - sidebarWidth - 3
	if m.panelW > 0 {
		w -= m.panelW + 1 // pane plus its divider
	}
	return max(w, 1)
}

// listHeight is the number of font rows the sidebar shows.
func (m Model) listHeight() int { return max(m.bodyHeight()-3, 1) }

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

// refreshPreview re-renders the preview. reset scrolls it to the top;
// otherwise the anchor font keeps its screen row while the fonts above it
// change height.
func (m *Model) refreshPreview(reset bool) {
	if !m.ready {
		return
	}
	anchor, hasAnchor := m.anchorFont()
	row := m.spans[anchor].start - m.preview.YOffset
	content, spans := m.previewContent()
	m.spans = spans
	m.preview.SetContent(content)
	switch {
	case reset:
		m.preview.GotoTop()
	case hasAnchor:
		if s, ok := m.spans[anchor]; ok {
			m.preview.SetYOffset(s.start - row)
		}
	}
}

// anchorFont is the cursor font when it is rendered, else the font at the
// top of the preview.
func (m Model) anchorFont() (string, bool) {
	if _, ok := m.spans[m.cursorName()]; ok {
		return m.cursorName(), true
	}
	y := m.preview.YOffset
	for name, s := range m.spans {
		if s.start <= y && y <= s.end {
			return name, true
		}
	}
	return "", false
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
			if m.state.Center {
				body = figfont.Center(body, f.Height, w)
			}
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
	divider := styles.divider.Render(strings.Repeat("│\n", m.bodyHeight()-1) + "│")
	cols := []string{
		m.sidebarView(),
		divider,
		lipgloss.NewStyle().PaddingLeft(1).Width(m.previewWidth() + 1).Render(m.preview.View()),
	}
	if m.panelW > 0 {
		cols = append(cols, divider, m.optionsView())
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
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
	return lipgloss.NewStyle().Width(sidebarWidth).Height(m.bodyHeight()).
		MaxHeight(m.bodyHeight()).Render(strings.Join(lines, "\n"))
}

func (m Model) countLine() string {
	n := 0
	for _, it := range m.items {
		if !it.IsHeader() {
			n++
		}
	}
	s := fmt.Sprintf("%d/%d fonts · %d selected", n, len(m.cat.Entries), m.state.SelectedCount())
	if m.loading {
		s += " · loading"
	}
	return truncate(s, sidebarWidth)
}

func (m Model) itemView(i int) string {
	it := m.items[i]
	var row string
	if it.IsHeader() {
		mark := styles.dim.Render("▸ ")
		if m.state.GroupSelected(it.Header) {
			mark = styles.selected.Render("● ")
		}
		count := fmt.Sprintf(" %d", it.Count)
		row = mark + styles.header.Render(truncate(it.Header, sidebarWidth-4-len(count))) + styles.dim.Render(count)
	} else {
		mark := styles.dim.Render("○ ")
		if m.state.IsSelected(it.Name) {
			mark = styles.selected.Render("● ")
		}
		dups := ""
		if it.Dups > 0 {
			dups = fmt.Sprintf(" +%d", it.Dups)
		}
		row = mark + highlight(truncate(it.Name, sidebarWidth-4-len(dups)), it.Matched) + styles.dim.Render(dups)
	}
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
		keys = "ctrl+o: options · type text · enter: new line · tab: search · pgup/pgdn: scroll · ctrl+c: quit"
	case focusSearch:
		keys = "ctrl+o: options · type to filter · ↑/↓: move · enter: select · esc: clear · tab: list"
	case focusList:
		keys = "ctrl+o: options · ↑/↓ j/k: move · space: select · x: clear selection · /: search · tab: text · q: quit"
	case focusOptions:
		keys = "↑/↓ j/k: move · space: toggle · ←/→: group by · tab: text · esc/ctrl+o: close options"
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
