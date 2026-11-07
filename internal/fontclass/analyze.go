// Package fontclass profiles fonts by how they look: exact-duplicate keys,
// four measured facet axes, and a visual fingerprint for similarity. It has
// no UI dependencies; all measures come from fixed probe strings, never the
// user's text.
package fontclass

import (
	"crypto/sha256"
	"math"
	"sort"
	"strings"

	"github.com/jnaO/figlet-display/internal/figfont"
)

// Axis is a facet axis.
type Axis int

const (
	Stroke Axis = iota
	Weight
	Size
	Slant
	NumAxes
)

// AxisInfo names an axis and its values in display order.
type AxisInfo struct {
	Axis   Axis
	Name   string
	Values []string
}

var axes = []AxisInfo{
	{Stroke, "Stroke", []string{"Line art", "Solid fill", "Block", "Box drawing", "Other Unicode"}},
	{Weight, "Weight", []string{"Light", "Regular", "Heavy"}},
	{Size, "Size", []string{"Tiny", "Small", "Medium", "Large", "Huge"}},
	{Slant, "Slant", []string{"Upright", "Slanted"}},
}

// Axes returns every axis with its values in display order.
func Axes() []AxisInfo { return axes }

// Stroke values, indexes into Axes()[Stroke].Values.
const (
	strokeLine = iota
	strokeFill
	strokeBlock
	strokeBox
	strokeOther
)

// Probe strings. Changing them changes every profile.
const (
	probe       = "HAMbgxo3"
	slantGlyphs = "HIlENTLFdk"
)

// Thresholds; .agent-docs/fontclass.md records how they were calibrated.
const (
	nonASCIIShare = 0.30 // a non-ASCII stroke class wins at this share of ink
	lightBelow    = 0.35 // density
	heavyFrom     = 0.55 // density
	slantedFrom   = 0.18 // slant score
	minSlantRows  = 2    // glyphs with fewer measured rows give no slant sample
)

// sizeUpTo maps ink rows to Size: Tiny ≤3, Small ≤5, Medium ≤8, Large ≤12, else Huge.
var sizeUpTo = []int{3, 5, 8, 12}

const horizontals = "_-=~"

const lineChars = "/\\|_-`',.:;()<>^~=\"![]{}"

// Profile is everything fontclass knows about one font.
type Profile struct {
	Key         [sha256.Size]byte
	Values      [NumAxes]int // index into Axes()[axis].Values
	Fingerprint []float64

	// Raw measures behind Values.
	Classes [5]float64 // share of ink per stroke class
	Density float64    // ink cells / bounding box of the probe
	InkRows int        // rows with ink in the probe
	Slant   float64    // median edge slope; positive leans right
}

// Value returns the profile's value name on axis a.
func (p Profile) Value(a Axis) string { return axes[a].Values[p.Values[a]] }

func strokeClass(r rune) int {
	switch {
	case r >= 0x2580 && r <= 0x259F:
		return strokeBlock
	case r >= 0x2500 && r <= 0x257F:
		return strokeBox
	case r > 126:
		return strokeOther
	case strings.ContainsRune(lineChars, r):
		return strokeLine
	default:
		return strokeFill
	}
}

// Analyze profiles f. It is deterministic.
func Analyze(f *figfont.Font) Profile {
	p := Profile{Key: f.GlyphKey()}
	rows := runeRows(inkRows(f.Render(probe, 0)))
	p.InkRows = len(rows)
	w := 0
	for _, r := range rows {
		w = max(w, len(r))
	}

	const gridH, gridW = 6, 32
	grid := make([]float64, gridH*gridW)
	cells := make([]float64, gridH*gridW)
	ink := 0
	for y, row := range rows {
		gy := y * gridH / max(p.InkRows, 1)
		for x := 0; x < w; x++ {
			gx := x * gridW / max(w, 1)
			cells[gy*gridW+gx]++
			if x < len(row) && row[x] != ' ' {
				ink++
				p.Classes[strokeClass(row[x])]++
				grid[gy*gridW+gx]++
			}
		}
	}
	if ink > 0 {
		for i := range p.Classes {
			p.Classes[i] /= float64(ink)
		}
	}
	if w*p.InkRows > 0 {
		p.Density = float64(ink) / float64(w*p.InkRows)
	}
	for i := range grid {
		if cells[i] > 0 {
			grid[i] /= cells[i]
		}
	}
	p.Slant = slantScore(f)

	p.Values[Stroke] = strokeValue(p.Classes)
	switch {
	case p.Density < lightBelow:
		p.Values[Weight] = 0
	case p.Density < heavyFrom:
		p.Values[Weight] = 1
	default:
		p.Values[Weight] = 2
	}
	p.Values[Size] = len(sizeUpTo)
	for i, limit := range sizeUpTo {
		if p.InkRows <= limit {
			p.Values[Size] = i
			break
		}
	}
	if p.Slant >= slantedFrom {
		p.Values[Slant] = 1
	}

	p.Fingerprint = fingerprint(p, w, grid)
	return p
}

func strokeValue(c [5]float64) int {
	best, share := -1, 0.0
	for _, k := range []int{strokeBlock, strokeBox, strokeOther} {
		if c[k] >= nonASCIIShare && c[k] > share {
			best, share = k, c[k]
		}
	}
	if best >= 0 {
		return best
	}
	if c[strokeLine] >= c[strokeFill] {
		return strokeLine
	}
	return strokeFill
}

// slantScore is the smaller of two edge measures, so a lean must show both
// with and without horizontal strokes (_ - = ~): counting them, a drop
// shadow's '_' reads as a lean; ignoring them, a bar row's far '|' does.
func slantScore(f *figfont.Font) float64 {
	return min(edgeSlant(f, false), edgeSlant(f, true))
}

// edgeSlant is the median over slantGlyphs of the mean left- and right-edge
// slope (ink column regressed on row, positive leaning right). Rows drawn
// only with horizontal strokes never count: a top bar of '_' sits beside the
// vertical stroke below it.
func edgeSlant(f *figfont.Font, skipHorizontals bool) float64 {
	var samples []float64
	for _, g := range slantGlyphs {
		rows := runeRows(inkRows(f.Render(string(g), 0)))
		var ys, left, right []float64
		for y, row := range rows {
			l, r, upright := -1, -1, false
			for x, c := range row {
				horizontal := strings.ContainsRune(horizontals, c)
				if c == ' ' || (skipHorizontals && horizontal) {
					continue
				}
				if l < 0 {
					l = x
				}
				r = x
				upright = upright || !horizontal
			}
			if upright {
				ys = append(ys, float64(y))
				left = append(left, float64(l))
				right = append(right, float64(r))
			}
		}
		if len(ys) < minSlantRows {
			continue
		}
		samples = append(samples, -(slope(ys, left)+slope(ys, right))/2)
	}
	if len(samples) == 0 {
		return 0
	}
	sort.Float64s(samples)
	return samples[len(samples)/2]
}

// slope regresses x on y.
func slope(ys, xs []float64) float64 {
	my, mx := mean(ys), mean(xs)
	num, den := 0.0, 0.0
	for i := range ys {
		num += (ys[i] - my) * (xs[i] - mx)
		den += (ys[i] - my) * (ys[i] - my)
	}
	if den == 0 {
		return 0
	}
	return num / den
}

func mean(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// fingerprint weights: stroke classes ×3, density ×2, log ink rows, log
// width per probe glyph per row, slant, and the ink grid ×0.5.
func fingerprint(p Profile, w int, grid []float64) []float64 {
	v := make([]float64, 0, 9+len(grid))
	for _, c := range p.Classes {
		v = append(v, 3*c)
	}
	h := float64(max(p.InkRows, 1))
	v = append(v,
		2*p.Density,
		math.Log(h),
		math.Log(float64(max(w, 1))/h/float64(len([]rune(probe)))),
		p.Slant,
	)
	for _, g := range grid {
		v = append(v, 0.5*g)
	}
	return v
}

func inkRows(lines []string) []string {
	s, e := 0, len(lines)
	for s < e && strings.TrimSpace(lines[s]) == "" {
		s++
	}
	for e > s && strings.TrimSpace(lines[e-1]) == "" {
		e--
	}
	return lines[s:e]
}

func runeRows(lines []string) [][]rune {
	out := make([][]rune, len(lines))
	for i, l := range lines {
		out[i] = []rune(l)
	}
	return out
}
