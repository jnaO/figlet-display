package figfont

// lineReader reproduces figlet's byte-level line reading.
type lineReader struct {
	b   []byte
	pos int
}

func (r *lineReader) readN(n int) []byte {
	end := min(r.pos+n, len(r.b))
	out := r.b[r.pos:end]
	r.pos = end
	return out
}

// fgets mirrors figlet's myfgets: it keeps up to n bytes, stopping after
// '\n'. It consumes (and drops) one extra byte when n runs out, and reports
// ok=false whenever end of input is reached, even after a partial line.
func (r *lineReader) fgets(n int) (line []byte, ok bool) {
	start := r.pos
	for {
		if r.pos >= len(r.b) {
			return r.b[start:r.pos], false
		}
		c := r.b[r.pos]
		r.pos++
		if n == 0 {
			return r.b[start : r.pos-1], true
		}
		n--
		if c == '\n' {
			return r.b[start:r.pos], true
		}
	}
}

func (r *lineReader) skipToEOL() {
	for r.pos < len(r.b) {
		c := r.b[r.pos]
		r.pos++
		if c == '\n' {
			return
		}
	}
}

// scanner implements the subset of sscanf used for the font header.
type scanner struct {
	b []byte
	i int
}

func (s *scanner) skipChar() bool {
	if s.i >= len(s.b) {
		return false
	}
	s.i++
	return true
}

func (s *scanner) char() (byte, bool) {
	if s.i >= len(s.b) {
		return 0, false
	}
	c := s.b[s.i]
	s.i++
	return c, true
}

// int reads an optionally signed decimal after skipping whitespace (%d).
func (s *scanner) int() (int, bool) {
	for s.i < len(s.b) && isSpace(rune(s.b[s.i])) {
		s.i++
	}
	j := s.i
	if j < len(s.b) && (s.b[j] == '-' || s.b[j] == '+') {
		j++
	}
	k := j
	v := 0
	for k < len(s.b) && s.b[k] >= '0' && s.b[k] <= '9' {
		v = v*10 + int(s.b[k]-'0')
		k++
	}
	if k == j {
		return 0, false
	}
	if s.b[s.i] == '-' {
		v = -v
	}
	s.i = k
	return v, true
}

// strtol parses a leading integer like C strtol with base 0 (decimal, 0octal,
// 0xhex, optional sign).
func strtol(b []byte) (int64, bool) {
	i := 0
	for i < len(b) && isSpace(rune(b[i])) {
		i++
	}
	neg := false
	if i < len(b) && (b[i] == '-' || b[i] == '+') {
		neg = b[i] == '-'
		i++
	}
	base := int64(10)
	if i+1 < len(b) && b[i] == '0' && (b[i+1] == 'x' || b[i+1] == 'X') && i+2 < len(b) && hexVal(b[i+2]) >= 0 {
		base = 16
		i += 2
	} else if i < len(b) && b[i] == '0' {
		base = 8
	}
	start := i
	var v int64
	for i < len(b) {
		d := hexVal(b[i])
		if d < 0 || int64(d) >= base {
			break
		}
		v = v*base + int64(d)
		i++
	}
	if i == start {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
