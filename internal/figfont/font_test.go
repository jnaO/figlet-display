package figfont

import (
	"os"
	"reflect"
	"testing"
)

func mustParse(t *testing.T, path string) *Font {
	t.Helper()
	f, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestParseHeader(t *testing.T) {
	cases := []struct {
		path      string
		hardblank rune
		layout    int
		dir       int
		toilet    bool
	}{
		{"testdata/tiny.flf", '$', 0, 0, false},
		{"testdata/tiny-zipped.flf", '$', 0, 0, false},
		{"testdata/tiny.tlf", 0x7F, 0, 0, true},
		{"testdata/tiny-rtl.flf", '$', 0, 1, false},
	}
	for _, c := range cases {
		f := mustParse(t, c.path)
		if f.Hardblank != c.hardblank || f.Layout != c.layout || f.PrintDirection != c.dir || f.Toilet != c.toilet || f.Height != 2 {
			t.Errorf("%s: got %+v", c.path, f)
		}
	}
}

func TestRender(t *testing.T) {
	cases := []struct {
		path, text string
		width      int
		want       []string
	}{
		{"testdata/tiny.flf", "ab", 0, []string{"a|b|", "a|b|"}},
		{"testdata/tiny.flf", "a b", 0, []string{"a| b|", "a| b|"}},
		{"testdata/tiny-zipped.flf", "ab", 0, []string{"a|b|", "a|b|"}},
		{"testdata/tiny-rtl.flf", "ab", 0, []string{"b|a|", "b|a|"}},
		{"testdata/tiny.tlf", "a☺", 0, []string{"a█:)", "a█()"}},
		{"testdata/tiny.flf", "aéb", 0, []string{"a|b|", "a|b|"}}, // missing char renders empty
		{"testdata/tiny.flf", "ab cd", 5, []string{"a|b|", "a|b|", "c|d|", "c|d|"}},
		{"testdata/tiny.flf", "abcd", 5, []string{"a|b|", "a|b|", "c|d|", "c|d|"}},
		{"testdata/tiny.flf", "", 0, nil},
	}
	for _, c := range cases {
		got := mustParse(t, c.path).Render(c.text, c.width)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %q w=%d: got %q, want %q", c.path, c.text, c.width, got, c.want)
		}
	}
}

func TestParseSkipsBOM(t *testing.T) {
	data, err := os.ReadFile("testdata/tiny.flf")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(append([]byte("\xEF\xBB\xBF"), data...))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Render("ab", 0); !reflect.DeepEqual(got, []string{"a|b|", "a|b|"}) {
		t.Fatalf("got %q", got)
	}
}

func TestParseRejectsNonFont(t *testing.T) {
	if _, err := Parse([]byte("hello\nworld\n")); err == nil {
		t.Fatal("expected error")
	}
}

func TestStrtol(t *testing.T) {
	cases := map[string]int64{"196": 196, "0x263A x": 0x263A, "0177": 127, "-5": -5, "  42 n": 42}
	for in, want := range cases {
		if got, ok := strtol([]byte(in)); !ok || got != want {
			t.Errorf("strtol(%q) = %d, %v", in, got, ok)
		}
	}
	if _, ok := strtol([]byte("nope")); ok {
		t.Error("expected no number")
	}
}
