package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jnaO/figlet-font-preview/internal/figfont"
)

const statusFor = 2 * time.Second

// writeClipboard puts text on the system clipboard (pbcopy on macOS).
var writeClipboard = clipboard.WriteAll

// copiedMsg reports a finished clipboard write.
type copiedMsg struct {
	name string
	err  error
}

// clearStatusMsg clears the footer status set with the same id.
type clearStatusMsg struct{ id int }

// copyCursor copies the cursor font's render as plain text: the preview's
// wrapping and centring, trimmed to the block's own width.
func (m *Model) copyCursor() tea.Cmd {
	name := m.cursorName()
	e, ok := m.cat.Get(name)
	if !ok {
		return m.setStatus("nothing to copy")
	}
	f, err := e.Font()
	if err != nil || f == nil {
		return m.setStatus(name + " is not loaded")
	}
	lines := f.Render(m.state.Text, m.previewWidth())
	if m.state.Center {
		lines = figfont.Center(lines, f.Height, m.previewWidth())
	}
	text := strings.Join(figfont.Trim(lines), "\n") + "\n"
	return func() tea.Msg { return copiedMsg{name, writeClipboard(text)} }
}

// setStatus shows s in the footer until statusFor passes.
func (m *Model) setStatus(s string) tea.Cmd {
	m.statusID++
	m.status = s
	id := m.statusID
	return tea.Tick(statusFor, func(time.Time) tea.Msg { return clearStatusMsg{id} })
}

func (m *Model) copied(msg copiedMsg) tea.Cmd {
	if msg.err != nil {
		return m.setStatus(fmt.Sprintf("copy failed: %v", msg.err))
	}
	return m.setStatus("copied " + msg.name)
}
