package fontclass

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jnaO/figlet-font-preview/internal/catalog"
	"github.com/jnaO/figlet-font-preview/internal/figfont"
)

// makeFont builds a FIGfont whose every printable glyph is rows(c).
func makeFont(t *testing.T, hardblank byte, layout string, height int, rows func(c rune) []string) *figfont.Font {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "flf2a%c %d %d 40 %s 1\ncomment\n", hardblank, height, height, layout)
	glyph := func(c rune) {
		r := rows(c)
		for i, row := range r {
			end := "@"
			if i == len(r)-1 {
				end = "@@"
			}
			b.WriteString(row + end + "\n")
		}
	}
	for c := rune(32); c <= 126; c++ {
		glyph(c)
	}
	for range 7 {
		glyph(0)
	}
	f, err := figfont.Parse([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// solid repeats ch in a w×h block, with hardblank hb for the space glyph.
func solid(ch string, w, h int, hb string) func(c rune) []string {
	return func(c rune) []string {
		fill := ch
		if c == ' ' || c == 0 {
			fill = hb
		}
		out := make([]string, h)
		for i := range out {
			out[i] = strings.Repeat(fill, w)
		}
		return out
	}
}

func TestGlyphKeyDuplicates(t *testing.T) {
	a := makeFont(t, '$', "-1", 3, solid("#", 3, 3, "$"))
	b := makeFont(t, '%', "-1", 3, solid("#", 3, 3, "%")) // different hardblank byte
	if a.GlyphKey() != b.GlyphKey() {
		t.Error("fonts differing only in hardblank byte should share a key")
	}
	c := makeFont(t, '$', "0", 3, solid("#", 3, 3, "$")) // different layout
	if a.GlyphKey() == c.GlyphKey() {
		t.Error("layout must change the key")
	}
	d := makeFont(t, '$', "-1", 3, func(r rune) []string {
		if r == 'A' {
			return []string{"###", "# #", "###"}
		}
		return solid("#", 3, 3, "$")(r)
	})
	if a.GlyphKey() == d.GlyphKey() {
		t.Error("one different glyph must change the key")
	}
}

func TestRepresentativeFollowsRoots(t *testing.T) {
	p := Profile{Key: [32]byte{1}}
	q := Profile{Key: [32]byte{2}}
	ix := New([]Input{{"Zed", 0, p}, {"alpha", 1, p}, {"beta", 1, p}, {"solo", 0, q}})
	if r := ix.Representative("alpha"); r != "Zed" {
		t.Errorf("representative = %q, want Zed (lowest root)", r)
	}
	if ix.Duplicates("Zed") != 2 || ix.Duplicates("alpha") != 0 || ix.Duplicates("solo") != 0 {
		t.Error("duplicate counts wrong")
	}
	if got := ix.Groups(); !reflect.DeepEqual(got, [][]string{{"Zed", "alpha", "beta"}}) {
		t.Errorf("groups = %v", got)
	}
	if ix.Representative("unknown") != "unknown" {
		t.Error("unprofiled font should represent itself")
	}
}

func TestAxisBoundaries(t *testing.T) {
	cases := []struct {
		name string
		font *figfont.Font
		axis Axis
		want string
	}{
		{"line chars", makeFont(t, '$', "-1", 4, solid("/", 4, 4, "$")), Stroke, "Line art"},
		{"fill chars", makeFont(t, '$', "-1", 4, solid("#", 4, 4, "$")), Stroke, "Solid fill"},
		{"block chars", makeFont(t, '$', "-1", 4, solid("█", 4, 4, "$")), Stroke, "Block"},
		{"box chars", makeFont(t, '$', "-1", 4, solid("═", 4, 4, "$")), Stroke, "Box drawing"},
		{"braille", makeFont(t, '$', "-1", 4, solid("⣿", 4, 4, "$")), Stroke, "Other Unicode"},
		{"solid is heavy", makeFont(t, '$', "-1", 4, solid("#", 4, 4, "$")), Weight, "Heavy"},
		{"3 rows tiny", makeFont(t, '$', "-1", 3, solid("#", 3, 3, "$")), Size, "Tiny"},
		{"4 rows small", makeFont(t, '$', "-1", 4, solid("#", 3, 4, "$")), Size, "Small"},
		{"8 rows medium", makeFont(t, '$', "-1", 8, solid("#", 3, 8, "$")), Size, "Medium"},
		{"12 rows large", makeFont(t, '$', "-1", 12, solid("#", 3, 12, "$")), Size, "Large"},
		{"13 rows huge", makeFont(t, '$', "-1", 13, solid("#", 3, 13, "$")), Size, "Huge"},
		{"upright", makeFont(t, '$', "-1", 6, solid("#", 3, 6, "$")), Slant, "Upright"},
		{"slanted", makeFont(t, '$', "-1", 6, func(c rune) []string {
			if c == ' ' || c == 0 {
				return solid("$", 8, 6, "$")(c)
			}
			out := make([]string, 6)
			for i := range out {
				out[i] = strings.Repeat(" ", 5-i) + "##" + strings.Repeat(" ", i+1)
			}
			return out
		}), Slant, "Slanted"},
	}
	for _, c := range cases {
		if got := Analyze(c.font).Value(c.axis); got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.name, axes[c.axis].Name, got, c.want)
		}
	}
	light := makeFont(t, '$', "-1", 4, func(c rune) []string { return []string{"#   ", "    ", "    ", "   #"} })
	if got := Analyze(light).Value(Weight); got != "Light" {
		t.Errorf("sparse glyphs: weight %q, want Light", got)
	}
}

func TestSimilarOrderAndTies(t *testing.T) {
	fp := func(v ...float64) Profile {
		return Profile{Key: [32]byte{byte(len(v)) + byte(v[0]*10)}, Fingerprint: v}
	}
	ix := New([]Input{
		{"anchor", 0, fp(0, 0)},
		{"far", 0, fp(5, 0)},
		{"near-b", 0, fp(1, 0)},
		{"near-a", 0, fp(0, 1)},
	})
	if got := ix.Similar("anchor"); !reflect.DeepEqual(got, []string{"near-a", "near-b", "far"}) {
		t.Errorf("similar = %v", got)
	}
	if ix.Similar("missing") != nil {
		t.Error("unprofiled anchor should give nil")
	}
}

func installedIndex(t *testing.T) *Index {
	t.Helper()
	c := catalog.Discover(catalog.SystemRoots)
	if len(c.Entries) == 0 {
		t.Skip("no installed fonts")
	}
	return Build(c)
}

func TestInstalledDuplicates(t *testing.T) {
	ix := installedIndex(t)
	want := [][]string{{"C64-fonts/gothic", "gothic__"}, {"battle_s", "battlesh"}, {"etcrvs__", "tec1____", "tec_7000"}}
	if got := ix.Groups(); !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %v, want %v", got, want)
	}
}

// calibration is hand-labelled from viewing each font; a threshold change
// that flips a label must be a deliberate decision.
var calibration = map[string]map[Axis]string{
	"standard":     {Stroke: "Line art", Weight: "Regular", Size: "Medium", Slant: "Upright"},
	"big":          {Stroke: "Line art", Weight: "Regular", Size: "Medium", Slant: "Upright"},
	"doom":         {Stroke: "Line art", Slant: "Upright"},
	"block":        {Stroke: "Line art", Weight: "Light", Slant: "Upright"},
	"small":        {Stroke: "Line art", Size: "Small", Slant: "Upright"},
	"starwars":     {Stroke: "Line art", Slant: "Upright"},
	"banner":       {Stroke: "Solid fill", Slant: "Upright"},
	"3-d":          {Stroke: "Solid fill", Weight: "Heavy", Slant: "Upright"},
	"colossal":     {Stroke: "Solid fill", Size: "Large", Slant: "Upright"},
	"fonts/roman":  {Stroke: "Solid fill", Slant: "Upright"},
	"helv":         {Stroke: "Solid fill", Slant: "Upright"},
	"cour":         {Stroke: "Solid fill", Slant: "Upright"},
	"sans":         {Stroke: "Solid fill", Slant: "Upright"},
	"times":        {Stroke: "Solid fill", Slant: "Upright"},
	"shadow":       {Stroke: "Line art", Slant: "Upright"},
	"smshadow":     {Slant: "Upright"},
	"mini":         {Slant: "Upright"},
	"straight":     {Slant: "Upright"},
	"term":         {Size: "Tiny", Weight: "Heavy"},
	"digital":      {Size: "Tiny"},
	"mono9":        {Stroke: "Block", Slant: "Upright"},
	"bigmono9":     {Stroke: "Block", Size: "Huge"},
	"smblock":      {Stroke: "Block"},
	"pagga":        {Stroke: "Block", Weight: "Heavy", Size: "Tiny"},
	"future":       {Stroke: "Box drawing"},
	"emboss":       {Stroke: "Box drawing"},
	"circle":       {Stroke: "Other Unicode"},
	"smbraille":    {Stroke: "Other Unicode"},
	"wideterm":     {Stroke: "Other Unicode"},
	"slant":        {Stroke: "Line art", Slant: "Slanted"},
	"lean":         {Weight: "Light", Slant: "Slanted"},
	"smslant":      {Slant: "Slanted"},
	"italic":       {Slant: "Slanted"},
	"5lineoblique": {Slant: "Slanted"},
	"slscript":     {Slant: "Slanted"},
	"alligator":    {Slant: "Slanted"},
	"hollywood":    {Slant: "Slanted"},
	"invita":       {Slant: "Slanted"},
	"ogre":         {Slant: "Slanted"},
	"speed":        {Slant: "Slanted"},
	"trek":         {Slant: "Slanted"},
	"weird":        {Slant: "Slanted"},
	"bigchief":     {Slant: "Slanted"},
	"eftitalic":    {Slant: "Slanted"},
	"helvi":        {Weight: "Light", Slant: "Slanted"},
	"couri":        {Slant: "Slanted"},
	"sbooki":       {Slant: "Slanted"},
	"eftiwater":    {Slant: "Slanted"},
	"pepper":       {Slant: "Slanted"},
}

func TestInstalledCalibration(t *testing.T) {
	ix := installedIndex(t)
	for name, labels := range calibration {
		p, ok := ix.Profile(name)
		if !ok {
			t.Errorf("%s: not installed or failed to parse", name)
			continue
		}
		for axis, want := range labels {
			if got := p.Value(axis); got != want {
				t.Errorf("%s: %s = %q, want %q (density %.2f, rows %d, slant %.2f)",
					name, axes[axis].Name, got, want, p.Density, p.InkRows, p.Slant)
			}
		}
	}
}

func TestInstalledNeighbours(t *testing.T) {
	ix := installedIndex(t)
	cases := []struct {
		anchor, near string
		within       int
	}{
		{"mono9", "smmono9", 5},
		{"mono9", "bigmono9", 5},
		{"banner", "banner3", 5},
		{"future", "emboss", 5},
		{"isometric1", "isometric2", 5},
		{"shadow", "smshadow", 3},
		{"script", "smscript", 3},
		{"starwars", "epic", 3},
		{"doom", "big", 5},
	}
	for _, c := range cases {
		top := ix.Similar(c.anchor)
		top = top[:min(c.within, len(top))]
		found := false
		for _, n := range top {
			found = found || n == c.near
		}
		if !found {
			t.Errorf("%s: %s not in top %d %v", c.anchor, c.near, c.within, top)
		}
	}
}

func TestInstalledDeterministic(t *testing.T) {
	a, b := installedIndex(t), installedIndex(t)
	for _, n := range a.names {
		pa, _ := a.Profile(n)
		pb, _ := b.Profile(n)
		if !reflect.DeepEqual(pa, pb) {
			t.Fatalf("%s: profiles differ between runs", n)
		}
	}
	if !reflect.DeepEqual(a.Similar("standard"), b.Similar("standard")) {
		t.Fatal("similar order differs between runs")
	}
}
