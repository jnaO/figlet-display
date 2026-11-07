package display

import (
	"reflect"
	"testing"

	"github.com/jnaO/figlet-display/internal/fontclass"
)

// profile builds a fixture profile: key groups duplicates, values per axis
// (Stroke, Weight, Size, Slant), fp is a 1-D fingerprint.
func profile(key byte, stroke, weight, size, slant int, fp float64) fontclass.Profile {
	return fontclass.Profile{
		Key:         [32]byte{key},
		Values:      [fontclass.NumAxes]int{stroke, weight, size, slant},
		Fingerprint: []float64{fp},
	}
}

// Fonts: Big/big are duplicates (Big in root 1, big in root 0).
var pipelineFonts = []string{"Big", "big", "block", "slant", "smslant", "unparsed"}

func pipelineState() *State {
	s := New(pipelineFonts, "hi")
	s.SetIndex(fontclass.New([]fontclass.Input{
		{Name: "Big", Root: 1, Profile: profile(1, 0, 1, 2, 0, 0.0)},
		{Name: "big", Root: 0, Profile: profile(1, 0, 1, 2, 0, 0.0)},
		{Name: "block", Root: 0, Profile: profile(2, 1, 0, 2, 0, 5.0)},
		{Name: "slant", Root: 0, Profile: profile(3, 0, 1, 2, 1, 1.0)},
		{Name: "smslant", Root: 0, Profile: profile(4, 0, 1, 1, 1, 2.0)},
	}))
	return s
}

func names(items []Item) []string {
	var out []string
	for _, it := range items {
		if it.IsHeader() {
			out = append(out, "#"+it.Header)
		} else {
			out = append(out, it.Name)
		}
	}
	return out
}

func TestOptionsOffMatchesPlainState(t *testing.T) {
	s := pipelineState()
	plain := New(pipelineFonts, "hi")
	if !reflect.DeepEqual(s.Sidebar(), plain.Sidebar()) || !reflect.DeepEqual(s.RenderSet(), plain.RenderSet()) {
		t.Fatal("an index with every option off must not change the output")
	}
}

func TestCollapse(t *testing.T) {
	s := pipelineState()
	s.Collapse = true
	items := s.Sidebar()
	if got := names(items); !reflect.DeepEqual(got, []string{"big", "block", "slant", "smslant", "unparsed"}) {
		t.Fatalf("got %v", got)
	}
	if items[0].Dups != 1 {
		t.Fatalf("big should carry +1, got %d", items[0].Dups)
	}
	s.Toggle("Big") // selected duplicates stay visible
	if got := names(s.Sidebar()); got[0] != "Big" {
		t.Fatalf("selected duplicate hidden: %v", got)
	}
}

func TestFacetFilters(t *testing.T) {
	s := pipelineState()
	s.ToggleFilter(fontclass.Slant, 1)
	if got := names(s.Sidebar()); !reflect.DeepEqual(got, []string{"slant", "smslant"}) {
		t.Fatalf("slanted: %v", got)
	}
	s.ToggleFilter(fontclass.Size, 2) // AND across axes
	if got := names(s.Sidebar()); !reflect.DeepEqual(got, []string{"slant"}) {
		t.Fatalf("slanted AND medium: %v", got)
	}
	s.ToggleFilter(fontclass.Size, 1) // OR within an axis
	if got := names(s.Sidebar()); !reflect.DeepEqual(got, []string{"slant", "smslant"}) {
		t.Fatalf("slanted AND (medium OR small): %v", got)
	}
	s.Toggle("block") // selected survives the filter
	if got := names(s.Sidebar()); !reflect.DeepEqual(got, []string{"block", "slant", "smslant"}) {
		t.Fatalf("selection dropped: %v", got)
	}
	if s.FacetCount(fontclass.Slant, 1) != 2 || s.FacetCount(fontclass.Stroke, 0) != 4 {
		t.Fatal("facet counts wrong")
	}
	s.ClearFilters()
	if s.HasFilters() {
		t.Fatal("filters not cleared")
	}
}

func TestSimilar(t *testing.T) {
	s := pipelineState()
	s.SetSimilar("slant")
	if got := names(s.Sidebar()); !reflect.DeepEqual(got, []string{"slant", "Big", "big", "smslant", "block"}) {
		t.Fatalf("similar order: %v", got)
	}
	s.Collapse = true
	if got := names(s.Sidebar()); !reflect.DeepEqual(got, []string{"slant", "big", "smslant", "block"}) {
		t.Fatalf("similar collapsed: %v", got)
	}
	s.ToggleFilter(fontclass.Weight, 1)
	s.Toggle("unparsed")
	if got := names(s.Sidebar()); !reflect.DeepEqual(got, []string{"slant", "big", "smslant", "unparsed"}) {
		t.Fatalf("similar filtered + selection: %v", got)
	}
	s.SetSimilar("")
	s.ClearFilters()
	s.Collapse = false
	if got := len(s.Sidebar()); got != len(pipelineFonts) {
		t.Fatalf("cleared similar should restore all fonts, got %d", got)
	}
}

func TestSimilarCap(t *testing.T) {
	var fonts []string
	var in []fontclass.Input
	for i := range SimilarCap + 10 {
		n := string(rune('A'+i/26)) + string(rune('a'+i%26))
		fonts = append(fonts, n)
		in = append(in, fontclass.Input{Name: n, Profile: profile(byte(i), 0, 0, 0, 0, float64(i))})
	}
	s := New(fonts, "")
	s.SetIndex(fontclass.New(in))
	s.SetSimilar(fonts[0])
	if got := len(s.Sidebar()); got != SimilarCap+1 {
		t.Fatalf("got %d items, want anchor + %d", got, SimilarCap)
	}
}

func TestGrouping(t *testing.T) {
	s := pipelineState()
	s.GroupBy = fontclass.Slant
	want := []string{"#Upright", "Big", "big", "block", "#Slanted", "slant", "smslant", "#" + Unclassified, "unparsed"}
	items := s.Sidebar()
	if got := names(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if items[0].Count != 3 {
		t.Fatalf("header count %d", items[0].Count)
	}
	if got := s.RenderSet(); !reflect.DeepEqual(got, []string{"Big", "big", "block", "slant", "smslant", "unparsed"}) {
		t.Fatalf("render set should follow sidebar order: %v", got)
	}
	s.ToggleGroup("Slanted")
	if !s.IsSelected("slant") || !s.IsSelected("smslant") || !s.GroupSelected("Slanted") {
		t.Fatal("group not selected")
	}
	s.Toggle("smslant")
	s.ToggleGroup("Slanted") // partly selected → select all
	if !s.GroupSelected("Slanted") {
		t.Fatal("partial group should become fully selected")
	}
	s.ToggleGroup("Slanted") // fully selected → deselect all
	if s.SelectedCount() != 0 {
		t.Fatal("group not deselected")
	}
}
