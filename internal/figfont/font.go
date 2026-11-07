// Package figfont parses FIGlet (flf2a) and TOIlet (tlf2a) fonts and renders
// text with them. The parser and renderer follow figlet 2.2.5's own behaviour,
// which is the test oracle (see .agent-docs/figfont.md).
package figfont

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

// Layout bits of the full_layout header field.
const (
	smEqual     = 1
	smLowline   = 2
	smHierarchy = 4
	smPair      = 8
	smBigX      = 16
	smHardblank = 32
	smKern      = 64
	smSmush     = 128
)

// figlet adds this slack to the header's max_length before reading glyph rows.
const maxLenSlack = 100

// figlet's MAXLEN, the buffer size for header and code-tag lines.
const figletMaxLine = 255

// utf8BOM is skipped before the signature; figlet itself rejects such files.
var utf8BOM = []byte("\xEF\xBB\xBF")

// deutsch lists the seven characters that follow 32..126 in every font.
var deutsch = [7]rune{196, 214, 220, 228, 246, 252, 223}

// Font is a parsed FIGfont.
type Font struct {
	Hardblank      rune
	Height         int
	Layout         int // full_layout, derived from old_layout when absent
	PrintDirection int // 0 left-to-right, 1 right-to-left
	Toilet         bool

	glyphs map[rune][][]rune
}

// ParseFile reads and parses the font at path.
func ParseFile(path string) (*Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// Parse parses font bytes, transparently unzipping a zip-archived font.
func Parse(data []byte) (*Font, error) {
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		var err error
		if data, err = unzipFirst(data); err != nil {
			return nil, err
		}
	}
	data = bytes.TrimPrefix(data, utf8BOM)
	return parse(&lineReader{b: data})
}

func unzipFirst(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("unzip: %w", err)
	}
	if len(zr.File) == 0 {
		return nil, errors.New("unzip: empty archive")
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		return nil, fmt.Errorf("unzip: %w", err)
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func parse(r *lineReader) (*Font, error) {
	magic := r.readN(4)
	header, _ := r.fgets(figletMaxLine + 1)
	if len(header) > 0 && header[len(header)-1] != '\n' {
		r.skipToEOL()
	}
	f := &Font{}
	switch string(magic) {
	case "flf2":
	case "tlf2":
		f.Toilet = true
	default:
		return nil, errors.New("not a FIGlet 2 font")
	}

	// sscanf(header, "%*c%c %d %*d %d %d %d %d %d")
	var maxLen, oldLayout, cmtLines, ffRight, fullLayout int
	s := scanner{b: header}
	n := 0
	if s.skipChar() {
		if hb, ok := s.char(); ok {
			f.Hardblank = rune(hb)
			n++
			fields := []*int{&f.Height, nil, &maxLen, &oldLayout, &cmtLines, &ffRight, &fullLayout}
			for _, dst := range fields {
				v, ok := s.int()
				if !ok {
					break
				}
				if dst != nil {
					*dst = v
					n++
				}
			}
		}
	}
	if n < 5 {
		return nil, errors.New("malformed header")
	}
	if n < 6 {
		ffRight = 0
	}
	if n < 7 {
		switch {
		case oldLayout == 0:
			fullLayout = smKern
		case oldLayout < 0:
			fullLayout = 0
		default:
			fullLayout = (oldLayout & 31) | smSmush
		}
	}
	if f.Height < 1 {
		f.Height = 1
	}
	if maxLen < 1 {
		maxLen = 1
	}
	maxLen += maxLenSlack
	f.Layout = fullLayout
	f.PrintDirection = ffRight

	for i := 0; i < cmtLines; i++ {
		r.skipToEOL()
	}

	// figlet's "missing character" is ord 0 with empty rows; a font may override it.
	f.glyphs = map[rune][][]rune{0: make([][]rune, f.Height)}
	for c := rune(' '); c <= '~'; c++ {
		f.glyphs[c] = f.readGlyph(r, maxLen)
	}
	for _, c := range deutsch {
		f.glyphs[c] = f.readGlyph(r, maxLen)
	}
	for {
		line, ok := r.fgets(maxLen + 1)
		if !ok {
			break
		}
		if len(line) > 0 && line[len(line)-1] != '\n' {
			r.skipToEOL()
		}
		code, ok := strtol(line)
		if !ok {
			code = 0
		}
		f.glyphs[rune(code)] = f.readGlyph(r, maxLen)
	}
	return f, nil
}

// readGlyph reads Height rows and strips trailing whitespace and endmarks.
func (f *Font) readGlyph(r *lineReader, maxLen int) [][]rune {
	rows := make([][]rune, f.Height)
	for i := range rows {
		line, ok := r.fgets(maxLen + 1)
		if !ok {
			line = nil
		}
		row := f.decode(line)
		k := len(row) - 1
		for k >= 0 && isSpace(row[k]) {
			k--
		}
		if k >= 0 {
			end := row[k]
			for k >= 0 && row[k] == end {
				k--
			}
		}
		rows[i] = row[:k+1]
	}
	return rows
}

// decode reads TOIlet rows as UTF-8. FIGlet rows are UTF-8 when valid,
// otherwise one rune per byte (Latin-1); figlet itself treats them as bytes.
func (f *Font) decode(line []byte) []rune {
	if !f.Toilet && !utf8.Valid(line) {
		out := make([]rune, len(line))
		for i, b := range line {
			out[i] = rune(b)
		}
		return out
	}
	out := make([]rune, 0, len(line))
	for len(line) > 0 {
		r, size := utf8.DecodeRune(line)
		line = line[size:]
		if r == utf8.RuneError && size == 1 {
			continue
		}
		out = append(out, r)
	}
	return out
}

// GlyphKey identifies fonts that render identically: a SHA-256 over height,
// layout, print direction and the rows of 32..126 plus the Deutsch
// characters, with the hardblank normalised so fonts using different
// hardblank bytes still match.
func (f *Font) GlyphKey() [sha256.Size]byte {
	h := sha256.New()
	var buf [4]byte
	put := func(v int32) {
		binary.LittleEndian.PutUint32(buf[:], uint32(v))
		h.Write(buf[:])
	}
	put(int32(f.Height))
	put(int32(f.Layout))
	put(int32(f.PrintDirection))
	glyph := func(c rune) {
		for _, row := range f.glyph(c) {
			put(int32(len(row)))
			for _, r := range row {
				if r == f.Hardblank {
					r = -1
				}
				put(r)
			}
		}
	}
	for c := rune(' '); c <= '~'; c++ {
		glyph(c)
	}
	for _, c := range deutsch {
		glyph(c)
	}
	var key [sha256.Size]byte
	h.Sum(key[:0])
	return key
}

// glyph returns the rows for c, falling back to the missing character.
func (f *Font) glyph(c rune) [][]rune {
	if g, ok := f.glyphs[c]; ok {
		return g
	}
	return f.glyphs[0]
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}
