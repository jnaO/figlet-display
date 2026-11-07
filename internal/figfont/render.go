package figfont

import "strings"

// Unlimited is the line width Render uses when width <= 0.
const Unlimited = 1 << 20

// Render draws text with the font, word-wrapping so no line is longer than
// width runes (width <= 0 means no limit). Hardblanks are output as spaces.
// The algorithm is figlet 2.2.5's (addchar/smushamt/smushem/splitline).
func (f *Font) Render(text string, width int) []string {
	if width <= 0 {
		width = Unlimited
	}
	r := &renderer{
		f:          f,
		mode:       f.Layout,
		rtl:        f.PrintDirection == 1,
		limit:      width,
		inputLimit: (width+1)*4 + 100,
	}
	r.clearLine()
	for _, c := range text {
		r.feed(c)
	}
	if r.outLen != 0 {
		r.printLine()
	}
	return r.lines
}

type renderer struct {
	f          *Font
	mode       int
	rtl        bool
	limit      int
	inputLimit int

	out    [][]rune
	outLen int
	input  []rune

	curr          [][]rune
	currW, prevW  int
	wordBreakMode int

	lines []string
}

func (r *renderer) clearLine() {
	r.out = make([][]rune, r.f.Height)
	r.outLen = 0
	r.input = r.input[:0]
}

// feed is figlet's main input loop body for one character.
func (r *renderer) feed(c rune) {
	switch c {
	case '\t', ' ', '\v', '\f', '\r':
		c = ' '
	case '\n':
	}
	if (c > 0 && c < ' ' && c != '\n') || c == 127 {
		return
	}
	for {
		retry := false
		if r.wordBreakMode == -1 {
			if c == ' ' {
				return
			}
			if c == '\n' {
				r.wordBreakMode = 0
				return
			}
			r.wordBreakMode = 0
		}
		switch {
		case c == '\n':
			r.printLine()
			r.wordBreakMode = 0
		case r.addChar(c):
			if c != ' ' {
				if r.wordBreakMode >= 2 {
					r.wordBreakMode = 3
				} else {
					r.wordBreakMode = 1
				}
			} else if r.wordBreakMode > 0 {
				r.wordBreakMode = 2
			} else {
				r.wordBreakMode = 0
			}
		case r.outLen == 0:
			for _, row := range r.curr {
				if r.rtl && len(row) > r.limit {
					row = row[len(row)-r.limit:]
				}
				r.putString(row)
			}
			r.wordBreakMode = -1
		case c == ' ':
			if r.wordBreakMode == 2 {
				r.splitLine()
			} else {
				r.printLine()
			}
			r.wordBreakMode = -1
		default:
			if r.wordBreakMode >= 2 {
				r.splitLine()
			} else {
				r.printLine()
			}
			if r.wordBreakMode == 3 {
				r.wordBreakMode = 1
			} else {
				r.wordBreakMode = 0
			}
			retry = true
		}
		if !retry {
			return
		}
	}
}

func (r *renderer) getLetter(c rune) {
	r.curr = r.f.glyph(c)
	r.prevW = r.currW
	r.currW = len(r.curr[0])
}

func (r *renderer) addChar(c rune) bool {
	r.getLetter(c)
	amt := r.smushAmount()
	if r.outLen+r.currW-amt > r.limit || len(r.input)+1 > r.inputLimit {
		return false
	}
	for row := range r.out {
		cur := r.curr[row]
		if r.rtl {
			line := append([]rune(nil), cur...)
			for k := 0; k < amt; k++ {
				i := r.currW - amt + k
				if i >= 0 && i < len(line) {
					line[i] = r.smushChars(line[i], at(r.out[row], k))
				}
			}
			line = cstr(line)
			if amt <= len(r.out[row]) {
				line = append(line, r.out[row][amt:]...)
			}
			r.out[row] = line
			continue
		}
		line := r.out[row]
		for k := 0; k < amt; k++ {
			i := r.outLen - amt + k
			if i >= 0 && i < len(line) {
				line[i] = r.smushChars(line[i], at(cur, k))
			}
		}
		line = cstr(line)
		if amt <= len(cur) {
			line = append(line, cur[amt:]...)
		}
		r.out[row] = line
	}
	r.outLen = len(r.out[0])
	r.input = append(r.input, c)
	return true
}

// smushAmount is figlet's smushamt: how far the current character can move
// into the output line.
func (r *renderer) smushAmount() int {
	if r.mode&(smSmush|smKern) == 0 {
		return 0
	}
	maxSmush := r.currW
	for row := range r.out {
		line, cur := r.out[row], r.curr[row]
		var ch1, ch2 rune
		var amt int
		if r.rtl {
			maxSmush = min(maxSmush, len(line))
			cb := len(cur)
			for ch1 = at(cur, cb); cb > 0 && (ch1 == 0 || ch1 == ' '); ch1 = at(cur, cb) {
				cb--
			}
			lb := 0
			for ch2 = at(line, lb); ch2 == ' '; ch2 = at(line, lb) {
				lb++
			}
			amt = lb + r.currW - 1 - cb
		} else {
			lb := len(line)
			for ch1 = at(line, lb); lb > 0 && (ch1 == 0 || ch1 == ' '); ch1 = at(line, lb) {
				lb--
			}
			cb := 0
			for ch2 = at(cur, cb); ch2 == ' '; ch2 = at(cur, cb) {
				cb++
			}
			amt = cb + r.outLen - 1 - lb
		}
		if ch1 == 0 || ch1 == ' ' {
			amt++
		} else if ch2 != 0 && r.smushChars(ch1, ch2) != 0 {
			amt++
		}
		maxSmush = min(maxSmush, amt)
	}
	// Uneven glyph rows can make this negative; figlet then reads before its
	// buffer (undefined behaviour). Clamping keeps the renderer safe.
	return max(maxSmush, 0)
}

// smushChars is figlet's smushem; 0 means the pair cannot be smushed.
func (r *renderer) smushChars(l, rc rune) rune {
	if l == ' ' {
		return rc
	}
	if rc == ' ' {
		return l
	}
	if r.prevW < 2 || r.currW < 2 {
		return 0
	}
	if r.mode&smSmush == 0 {
		return 0
	}
	hb := r.f.Hardblank
	if r.mode&63 == 0 {
		if l == hb {
			return rc
		}
		if rc == hb {
			return l
		}
		if r.rtl {
			return l
		}
		return rc
	}
	if r.mode&smHardblank != 0 && l == hb && rc == hb {
		return l
	}
	if l == hb || rc == hb {
		return 0
	}
	if r.mode&smEqual != 0 && l == rc {
		return l
	}
	if r.mode&smLowline != 0 {
		if l == '_' && in("|/\\[]{}()<>", rc) {
			return rc
		}
		if rc == '_' && in("|/\\[]{}()<>", l) {
			return l
		}
	}
	if r.mode&smHierarchy != 0 {
		classes := []string{"|", "/\\", "[]", "{}", "()", "<>"}
		for i := 0; i < len(classes)-1; i++ {
			higher := strings.Join(classes[i+1:], "")
			if in(classes[i], l) && in(higher, rc) {
				return rc
			}
			if in(classes[i], rc) && in(higher, l) {
				return l
			}
		}
	}
	if r.mode&smPair != 0 {
		for _, p := range []string{"[]", "{}", "()"} {
			o, c := rune(p[0]), rune(p[1])
			if (l == o && rc == c) || (rc == o && l == c) {
				return '|'
			}
		}
	}
	if r.mode&smBigX != 0 {
		switch {
		case l == '/' && rc == '\\':
			return '|'
		case rc == '/' && l == '\\':
			return 'Y'
		case l == '>' && rc == '<':
			return 'X'
		}
	}
	return 0
}

func (r *renderer) splitLine() {
	gotSpace := false
	lastSpace := len(r.input) - 1
	i := len(r.input) - 1
	for ; i >= 0; i-- {
		if !gotSpace && r.input[i] == ' ' {
			gotSpace = true
			lastSpace = i
		}
		if gotSpace && r.input[i] != ' ' {
			break
		}
	}
	part1 := append([]rune(nil), r.input[:i+1]...)
	part2 := append([]rune(nil), r.input[lastSpace+1:]...)
	r.clearLine()
	for _, c := range part1 {
		r.addChar(c)
	}
	r.printLine()
	for _, c := range part2 {
		r.addChar(c)
	}
}

func (r *renderer) printLine() {
	for _, row := range r.out {
		r.putString(row)
	}
	r.clearLine()
}

func (r *renderer) putString(row []rune) {
	if len(row) > r.limit {
		row = row[:r.limit]
	}
	var b strings.Builder
	for _, c := range row {
		if c == r.f.Hardblank {
			c = ' '
		}
		b.WriteRune(c)
	}
	r.lines = append(r.lines, b.String())
}

// at returns s[i], or 0 (C's terminator) outside the slice.
func at(s []rune, i int) rune {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// cstr truncates at the first 0, as C string functions would see it.
func cstr(s []rune) []rune {
	for i, c := range s {
		if c == 0 {
			return s[:i]
		}
	}
	return s
}

// in mirrors strchr(set, c) != NULL, which also matches the terminator.
func in(set string, c rune) bool {
	return c == 0 || strings.ContainsRune(set, c)
}
