// Package display holds the UI-agnostic app state: sample text, fuzzy font
// search, the selection, the grouping options, and which fonts to render.
// Front-ends drive it.
package display

import (
	"sort"

	"github.com/sahilm/fuzzy"

	"github.com/jnaO/figlet-font-preview/internal/fontclass"
)

// NoGroup is GroupBy's value for an ungrouped sidebar.
const NoGroup fontclass.Axis = -1

// SimilarCap is how many look-alikes follow the Similar anchor.
const SimilarCap = 30

// Unclassified labels the group of fonts without a profile.
const Unclassified = "Unclassified"

// Item is a sidebar row: a font, or a group header when Header is set.
type Item struct {
	Name    string // font name; empty for a header
	Matched []int  // rune indexes the query matched
	Dups    int    // other fonts collapsed into this one
	Header  string // group label
	Count   int    // fonts in the header's group
}

// IsHeader reports whether the item is a group header.
func (it Item) IsHeader() bool { return it.Header != "" }

// State is the app state over a fixed, sorted list of font names.
type State struct {
	Text     string
	Center   bool           // centre each rendered line block in the preview width
	Collapse bool           // show one representative per exact-duplicate group
	GroupBy  fontclass.Axis // NoGroup, or the axis the sidebar is grouped by

	query    string
	fonts    []string
	selected map[string]bool
	index    *fontclass.Index
	filters  [fontclass.NumAxes]map[int]bool
	similar  string
}

// New returns a state over fonts (expected sorted) with the given text.
func New(fonts []string, text string) *State {
	return &State{Text: text, fonts: fonts, selected: map[string]bool{}, GroupBy: NoGroup}
}

func (s *State) Query() string                         { return s.query }
func (s *State) SetQuery(q string)                     { s.query = q }
func (s *State) Fonts() []string                       { return s.fonts }
func (s *State) IsSelected(n string) bool              { return s.selected[n] }
func (s *State) SelectedCount() int                    { return len(s.selected) }
func (s *State) ClearSelection()                       { s.selected = map[string]bool{} }
func (s *State) SetIndex(ix *fontclass.Index)          { s.index = ix }
func (s *State) Index() *fontclass.Index               { return s.index }
func (s *State) Similar() string                       { return s.similar }
func (s *State) SetSimilar(name string)                { s.similar = name }
func (s *State) Filtered(a fontclass.Axis, v int) bool { return s.filters[a][v] }

// Toggle flips the selection of a font.
func (s *State) Toggle(name string) {
	if s.selected[name] {
		delete(s.selected, name)
	} else {
		s.selected[name] = true
	}
}

// ToggleFilter flips one facet value. Values combine with OR within an axis
// and AND across axes.
func (s *State) ToggleFilter(a fontclass.Axis, v int) {
	if s.filters[a] == nil {
		s.filters[a] = map[int]bool{}
	}
	if s.filters[a][v] {
		delete(s.filters[a], v)
	} else {
		s.filters[a][v] = true
	}
}

// ClearFilters removes every facet filter.
func (s *State) ClearFilters() { s.filters = [fontclass.NumAxes]map[int]bool{} }

// HasFilters reports whether any facet filter is active.
func (s *State) HasFilters() bool {
	for _, f := range s.filters {
		if len(f) > 0 {
			return true
		}
	}
	return false
}

func (s *State) matchesFilters(name string) bool {
	p, ok := s.index.Profile(name)
	if !ok {
		return false
	}
	for a, f := range s.filters {
		if len(f) > 0 && !f[p.Values[a]] {
			return false
		}
	}
	return true
}

// FacetCount counts the fonts with value v on axis a, honouring Collapse
// but not the facet filters.
func (s *State) FacetCount(a fontclass.Axis, v int) int {
	if s.index == nil {
		return 0
	}
	n := 0
	for _, f := range s.collapsed() {
		if got, ok := s.index.Value(f, a); ok && got == v {
			n++
		}
	}
	return n
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

func (s *State) collapsed() []string {
	if s.index == nil || !s.Collapse {
		return s.fonts
	}
	return keep(s.fonts, func(n string) bool { return s.index.Representative(n) == n })
}

// universe is the font list the render-set rule runs over: collapse, then
// facet filters, then the Similar anchor and its look-alikes; selected fonts
// are always added back.
func (s *State) universe() []string {
	if s.index == nil {
		return s.fonts
	}
	u := s.collapsed()
	if s.HasFilters() {
		u = keep(u, s.matchesFilters)
	}
	if s.similar != "" {
		in := set(u)
		list := []string{s.similar}
		for _, n := range s.index.Similar(s.similar) {
			if len(list) > SimilarCap {
				break
			}
			if in[n] {
				list = append(list, n)
			}
		}
		u = list
	}
	in := set(u)
	var missing []string
	for _, n := range s.Selected() {
		if !in[n] {
			missing = append(missing, n)
		}
	}
	switch {
	case len(missing) == 0:
		return u
	case s.similar != "":
		return append(u, missing...)
	default:
		return keep(s.fonts, func(n string) bool { return in[n] || s.selected[n] })
	}
}

// Sidebar lists the universe with no query, otherwise its fuzzy matches in
// rank order. With GroupBy set, a header precedes each non-empty group.
func (s *State) Sidebar() []Item {
	u := s.universe()
	var items []Item
	if s.query == "" {
		items = make([]Item, len(u))
		for i, n := range u {
			items[i] = Item{Name: n}
		}
	} else {
		for _, m := range fuzzy.Find(s.query, u) {
			items = append(items, Item{Name: m.Str, Matched: m.MatchedIndexes})
		}
	}
	if s.index != nil && s.Collapse {
		for i := range items {
			items[i].Dups = s.index.Duplicates(items[i].Name)
		}
	}
	if s.index == nil || s.GroupBy == NoGroup {
		return items
	}
	return s.group(items)
}

func (s *State) group(items []Item) []Item {
	values := fontclass.Axes()[s.GroupBy].Values
	buckets := make([][]Item, len(values)+1) // last: Unclassified
	for _, it := range items {
		v, ok := s.index.Value(it.Name, s.GroupBy)
		if !ok {
			v = len(values)
		}
		buckets[v] = append(buckets[v], it)
	}
	out := make([]Item, 0, len(items)+len(buckets))
	for i, b := range buckets {
		if len(b) == 0 {
			continue
		}
		label := Unclassified
		if i < len(values) {
			label = values[i]
		}
		out = append(out, Item{Header: label, Count: len(b)})
		out = append(out, b...)
	}
	return out
}

// ToggleGroup selects every font under the header label, or deselects them
// all when they are already all selected.
func (s *State) ToggleGroup(label string) {
	var members []string
	in := false
	for _, it := range s.Sidebar() {
		if it.IsHeader() {
			in = it.Header == label
			continue
		}
		if in {
			members = append(members, it.Name)
		}
	}
	all := len(members) > 0
	for _, n := range members {
		all = all && s.selected[n]
	}
	for _, n := range members {
		if all {
			delete(s.selected, n)
		} else {
			s.selected[n] = true
		}
	}
}

// GroupSelected reports whether every font under the header label is selected.
func (s *State) GroupSelected(label string) bool {
	in, any := false, false
	for _, it := range s.Sidebar() {
		if it.IsHeader() {
			in = it.Header == label
			continue
		}
		if in {
			if !s.selected[it.Name] {
				return false
			}
			any = true
		}
	}
	return any
}

// RenderSet returns the fonts to render, in order:
//   - no query, no selection: the sidebar's fonts in sidebar order
//   - no query: the selection
//   - query: the selection, then matches not already listed
func (s *State) RenderSet() []string {
	if s.query == "" && len(s.selected) > 0 {
		return s.Selected()
	}
	var out []string
	if s.query != "" {
		out = s.Selected()
	}
	for _, it := range s.Sidebar() {
		if !it.IsHeader() && !s.selected[it.Name] {
			out = append(out, it.Name)
		}
	}
	return out
}

func keep(names []string, ok func(string) bool) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if ok(n) {
			out = append(out, n)
		}
	}
	return out
}

func set(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}
