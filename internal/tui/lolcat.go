package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Rainbow parameters, equivalent to `lolcat -S 20 -p 7 -F 0.05`.
const (
	lolSeed   = 20
	lolSpread = 7.0
	lolFreq   = 0.05
)

// rainbow is lolcat's colour function.
func rainbow(i float64) lipgloss.Color {
	c := func(phase float64) int { return int(math.Sin(lolFreq*i+phase)*127 + 128) }
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c(0), c(2*math.Pi/3), c(4*math.Pi/3)))
}

// lolcat colours each rune of lines like lolcat (row r, column c gets
// rainbow(seed + r + c/spread)) on the tint background, padding every line to
// width columns.
func lolcat(lines []string, width int) []string {
	out := make([]string, len(lines))
	for row, line := range lines {
		var b strings.Builder
		col := 0
		for _, r := range line {
			st := styles.tint
			if r != ' ' {
				st = st.Foreground(rainbow(lolSeed + float64(row) + float64(col)/lolSpread))
			}
			b.WriteString(st.Render(string(r)))
			col++
		}
		b.WriteString(tintPad(width - lipgloss.Width(line)))
		out[row] = b.String()
	}
	return out
}

// tintLine renders an already-plain line on the tint background, padded.
func tintLine(s string, width int) string {
	return styles.tint.Render(s) + tintPad(width-lipgloss.Width(s))
}

func tintPad(n int) string {
	if n <= 0 {
		return ""
	}
	return styles.tint.Render(strings.Repeat(" ", n))
}
