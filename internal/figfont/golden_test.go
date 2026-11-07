package figfont_test

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jnaO/figlet-font-preview/internal/catalog"
)

const goldenSample = `Hello World 0123456789 !?.,:;-+=()[]{}<>/\|_@#$%^&*'"~ abcxyz ABCXYZ`

// undefinedGlyphs lists, per font, glyphs for which figlet's own output is
// undefined behaviour, so they are left out of that font's sample:
//   - gradient: the file is misaligned from '}' on, giving glyphs with uneven
//     row widths; figlet then reads memory before its buffer.
//   - pyramid: the hardblank is the byte 0xFF, which figlet mishandles in the
//     space glyph and prints uninitialised memory.
var undefinedGlyphs = map[string]string{
	"gradient": "}~",
	"pyramid":  " ",
}

func sampleFor(name string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(undefinedGlyphs[name], r) {
			return -1
		}
		return r
	}, goldenSample)
}

// installed returns the installed catalog, skipping when fonts or the figlet
// oracle are unavailable.
func installed(t testing.TB) *catalog.Catalog {
	t.Helper()
	if _, err := exec.LookPath("figlet"); err != nil {
		t.Skip("figlet binary not installed")
	}
	c := catalog.Discover(catalog.SystemRoots)
	if len(c.Entries) == 0 {
		t.Skip("no installed fonts")
	}
	return c
}

// figlet renders text with the real binary; width is our max line length.
func figlet(t *testing.T, path, text string, width int) string {
	t.Helper()
	dir, file := filepath.Split(path)
	name := strings.TrimSuffix(file, filepath.Ext(file))
	out, err := exec.Command("figlet", "-l", "-w", strconv.Itoa(width+1), "-d", dir, "-f", name, "--", text).Output()
	if err != nil {
		t.Fatalf("figlet %s: %v", name, err)
	}
	return string(out)
}

func ours(t *testing.T, e *catalog.Entry, text string, width int) string {
	t.Helper()
	f, err := e.Load()
	if err != nil {
		t.Fatal(err)
	}
	lines := f.Render(text, width)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestGoldenAllFonts(t *testing.T) {
	c := installed(t)
	for _, e := range c.Entries {
		t.Run(e.Name, func(t *testing.T) {
			t.Parallel()
			sample := sampleFor(e.Name)
			want := figlet(t, e.Path, sample, 99999)
			if got := ours(t, e, sample, 99999); got != want {
				t.Errorf("mismatch\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
}

func TestGoldenWrapped(t *testing.T) {
	c := installed(t)
	for _, e := range c.Entries {
		t.Run(e.Name, func(t *testing.T) {
			t.Parallel()
			sample := sampleFor(e.Name)
			for _, w := range []int{40, 79} {
				want := figlet(t, e.Path, sample, w)
				if got := ours(t, e, sample, w); got != want {
					t.Errorf("width %d mismatch\n--- got\n%s\n--- want\n%s", w, got, want)
				}
			}
		})
	}
}

func BenchmarkRenderAll(b *testing.B) {
	c := installed(b)
	for _, e := range c.Entries {
		if _, err := e.Load(); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for b.Loop() {
		for _, e := range c.Entries {
			f, _ := e.Load()
			f.Render("Hello World", 120)
		}
	}
}
