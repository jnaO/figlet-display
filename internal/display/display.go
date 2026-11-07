// Package display holds the UI-agnostic app state: sample text, fuzzy font
// search, the selection, and which fonts to render. Front-ends drive it.
package display

import (
	"sort"

	"github.com/sahilm/fuzzy"
)

// Item is a sidebar row: a font name and the rune indexes the query matched.
type Item struct {
	Name    string
	Matched []int
}

// State is the app state over a fixed, sorted list of font names.
type State struct {
	Text     string
	query    string
	fonts    []string
	selected map[string]bool
}

// New returns a state over fonts (expected sorted) with the given text.
func New(fonts []string, text string) *State {
	return &State{Text: text, fonts: fonts, selected: map[string]bool{}}
}

func (s *State) Query() string            { return s.query }
func (s *State) SetQuery(q string)        { s.query = q }
func (s *State) Fonts() []string          { return s.fonts }
func (s *State) IsSelected(n string) bool { return s.selected[n] }
func (s *State) SelectedCount() int       { return len(s.selected) }
func (s *State) ClearSelection()          { s.selected = map[string]bool{} }

// Toggle flips the selection of a font.
func (s *State) Toggle(name string) {
	if s.selected[name] {
		delete(s.selected, name)
	} else {
		s.selected[name] = true
	}
}

// Selected returns the selected fonts sorted by name.
func (s *State) Selected() []string {
	out := make([]string, 0, len(s.selected))
	for n := range s.selected {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Sidebar lists all fonts with no query, otherwise the fuzzy matches in rank
// order.
func (s *State) Sidebar() []Item {
	if s.query == "" {
		items := make([]Item, len(s.fonts))
		for i, n := range s.fonts {
			items[i] = Item{Name: n}
		}
		return items
	}
	matches := fuzzy.Find(s.query, s.fonts)
	items := make([]Item, len(matches))
	for i, m := range matches {
		items[i] = Item{Name: m.Str, Matched: m.MatchedIndexes}
	}
	return items
}

// RenderSet returns the fonts to render, in order:
//   - no query, no selection: all fonts
//   - no query: the selection
//   - query: the selection, then matches not already listed
func (s *State) RenderSet() []string {
	if s.query == "" {
		if len(s.selected) == 0 {
			return s.fonts
		}
		return s.Selected()
	}
	out := s.Selected()
	for _, it := range s.Sidebar() {
		if !s.selected[it.Name] {
			out = append(out, it.Name)
		}
	}
	return out
}
