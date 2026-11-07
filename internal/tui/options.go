package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jnaO/figlet-font-preview/internal/catalog"
	"github.com/jnaO/figlet-font-preview/internal/display"
	"github.com/jnaO/figlet-font-preview/internal/fontclass"
)

type optKind int

const (
	optCenter optKind = iota
	optCollapse
	optGroupBy
	optSimilar
	optAxis // non-selectable axis label
	optFacet
	optClear
)

// optRow is one row of the options panel.
type optRow struct {
	kind  optKind
	axis  fontclass.Axis
	value int
}

func (r optRow) selectable() bool { return r.kind != optAxis }

const (
	optionsWidth = 32 // options pane content columns, excluding its divider
	slideStep    = 8  // columns the pane grows or shrinks per frame
	slideEvery   = 16 * time.Millisecond
)

// slideMsg advances the options pane's slide in or out.
type slideMsg struct{}

func slide() tea.Cmd {
	return tea.Tick(slideEvery, func(time.Time) tea.Msg { return slideMsg{} })
}

// toggleOptions opens the options pane (taking focus) or closes it
// (returning focus to where it was); the pane slides in from the right.
func (m *Model) toggleOptions() tea.Cmd {
	if m.panelOpen {
		m.panelOpen = false
		return tea.Batch(m.setFocus(m.prevFocus), slide())
	}
	m.panelOpen = true
	m.prevFocus = m.focus
	m.moveOption(0)
	return tea.Batch(m.setFocus(focusOptions), slide())
}

// stepSlide moves the pane width one frame toward its target.
func (m *Model) stepSlide() tea.Cmd {
	target := 0
	if m.panelOpen {
		target = optionsWidth
	}
	switch {
	case m.panelW < target:
		m.panelW = min(m.panelW+slideStep, target)
	case m.panelW > target:
		m.panelW = max(m.panelW-slideStep, target)
	default:
		return nil
	}
	m.layout()
	m.refreshPreview(false)
	if m.panelW == target {
		return nil
	}
	return slide()
}

// indexMsg delivers the fontclass index built after every font has loaded.
type indexMsg struct{ index *fontclass.Index }

func buildIndex(cat *catalog.Catalog) tea.Cmd {
	return func() tea.Msg { return indexMsg{fontclass.Build(cat)} }
}

// optionRows lists the panel rows in display order.
func optionRows() []optRow {
	rows := []optRow{{kind: optCenter}, {kind: optCollapse}, {kind: optGroupBy}, {kind: optSimilar}}
	for _, a := range fontclass.Axes() {
		rows = append(rows, optRow{kind: optAxis, axis: a.Axis})
		for v := range a.Values {
			rows = append(rows, optRow{kind: optFacet, axis: a.Axis, value: v})
		}
	}
	return append(rows, optRow{kind: optClear})
}

func (m Model) handlePanelKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		return m, m.toggleOptions()
	case "up", "k":
		m.moveOption(-1)
	case "down", "j":
		m.moveOption(1)
	case " ", "enter", "right", "l":
		m.applyOption(optionRows()[m.optCursor], 1)
	case "left", "h":
		m.applyOption(optionRows()[m.optCursor], -1)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// moveOption moves the panel cursor by d selectable rows (0 just snaps it
// onto a selectable row) and keeps it in view.
func (m *Model) moveOption(d int) {
	rows := optionRows()
	step := 1
	if d < 0 {
		step = -1
	}
	i := max(0, min(m.optCursor, len(rows)-1))
	for moved := 0; moved < max(d, -d); moved++ {
		j := i + step
		for j >= 0 && j < len(rows) && !rows[j].selectable() {
			j += step
		}
		if j < 0 || j >= len(rows) {
			break
		}
		i = j
	}
	for !rows[i].selectable() {
		i++
	}
	m.optCursor = i
	h := m.optionsHeight()
	if m.optCursor < m.optOffset {
		m.optOffset = m.optCursor
		if m.optCursor > 0 && !rows[m.optCursor-1].selectable() {
			m.optOffset-- // keep the axis label above its first value
		}
	}
	if m.optCursor >= m.optOffset+h {
		m.optOffset = m.optCursor - h + 1
	}
}

// applyOption toggles a row; dir only matters for Group by.
func (m *Model) applyOption(r optRow, dir int) {
	s := m.state
	ready := s.Index() != nil
	switch r.kind {
	case optCenter:
		if dir < 0 {
			return
		}
		s.Center = !s.Center
		m.refreshPreview(false)
		return
	case optCollapse:
		if !ready || dir < 0 {
			return
		}
		s.Collapse = !s.Collapse
	case optGroupBy:
		if !ready {
			return
		}
		n := int(fontclass.NumAxes) + 1 // NoGroup plus each axis
		s.GroupBy = fontclass.Axis((int(s.GroupBy)+1+dir+n)%n - 1)
	case optSimilar:
		if !ready || dir < 0 {
			return
		}
		switch {
		case s.Similar() != "":
			s.SetSimilar("")
		case m.cursorName() != "":
			s.SetSimilar(m.cursorName())
		default:
			return
		}
	case optFacet:
		if !ready || dir < 0 {
			return
		}
		s.ToggleFilter(r.axis, r.value)
	case optClear:
		if !ready || dir < 0 {
			return
		}
		s.ClearFilters()
	default:
		return
	}
	m.itemsChanged()
}

// itemsChanged rebuilds the sidebar after the universe changed, keeping the
// cursor on the same font when it is still listed.
func (m *Model) itemsChanged() {
	name := m.cursorName()
	m.items = m.state.Sidebar()
	m.cursor = 0
	for i, it := range m.items {
		if name != "" && it.Name == name {
			m.cursor = i
			break
		}
	}
	m.clampOffset()
	m.refreshPreview(false)
	m.revealCursor()
}

// optionsHeight is the number of option rows the pane shows.
func (m Model) optionsHeight() int { return max(m.bodyHeight()-2, 1) }

// optionsView renders the pane at its current (possibly sliding) width.
func (m Model) optionsView() string {
	title := styles.label.Render("Options")
	if m.focus != focusOptions {
		title = styles.header.Render("Options")
	}
	lines := []string{" " + title + styles.dim.Render("  ctrl+o"), styles.rule.Render(strings.Repeat("─", optionsWidth))}
	rows := optionRows()
	end := min(m.optOffset+m.optionsHeight(), len(rows))
	for i := m.optOffset; i < end; i++ {
		lines = append(lines, m.optionLine(rows[i], i == m.optCursor && m.focus == focusOptions))
	}
	return lipgloss.NewStyle().Width(m.panelW).MaxWidth(m.panelW).
		Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(strings.Join(lines, "\n"))
}

func (m Model) optionLine(r optRow, cursor bool) string {
	s := m.state
	ready := s.Index() != nil
	check := func(on bool) string {
		if on {
			return styles.selected.Render("[x]")
		}
		return styles.dim.Render("[ ]")
	}
	pending := styles.dim.Render("analysing…")
	var line string
	switch r.kind {
	case optCenter:
		line = check(s.Center) + " Center text"
	case optCollapse:
		line = check(s.Collapse) + " Collapse duplicates"
		if !ready {
			line = "    Collapse  " + pending
		}
	case optGroupBy:
		group := "none"
		if s.GroupBy != display.NoGroup {
			group = fontclass.Axes()[s.GroupBy].Name
		}
		line = "    Group by ‹ " + styles.label.Render(group) + " ›"
		if !ready {
			line = "    Group by  " + pending
		}
	case optSimilar:
		switch {
		case !ready:
			line = "    Similar   " + pending
		case s.Similar() != "":
			line = check(true) + " Similar to " + truncate(s.Similar(), optionsWidth-17)
		case m.cursorName() != "":
			line = check(false) + " Similar to " + truncate(m.cursorName(), optionsWidth-17)
		default:
			line = styles.dim.Render("    Similar to: pick a font")
		}
	case optAxis:
		return "  " + styles.header.Render(fontclass.Axes()[r.axis].Name)
	case optFacet:
		name := fontclass.Axes()[r.axis].Values[r.value]
		if !ready {
			line = "    " + styles.dim.Render(name)
		} else {
			count := fmt.Sprint(s.FacetCount(r.axis, r.value))
			pad := max(optionsWidth-8-len([]rune(name))-len(count), 1)
			line = check(s.Filtered(r.axis, r.value)) + " " + name + strings.Repeat(" ", pad) + styles.dim.Render(count)
		}
	case optClear:
		line = "    Clear filters"
		if !s.HasFilters() {
			line = styles.dim.Render(line)
		}
	}
	if cursor {
		return styles.cursor.Render("›") + " " + line
	}
	return "  " + line
}
