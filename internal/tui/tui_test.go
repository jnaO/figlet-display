package tui

import "testing"

func TestRevealOffset(t *testing.T) {
	const h = 10
	cases := []struct {
		name             string
		y, start, end, w int
	}{
		{"fully visible stays", 0, 2, 8, 0},
		{"exactly fits at bottom stays", 0, 4, 10, 0},
		{"below scrolls just enough", 0, 8, 14, 4},
		{"above scrolls to its top", 20, 12, 18, 12},
		{"taller than view shows top", 0, 5, 30, 5},
		{"taller than view above shows top", 40, 5, 30, 5},
	}
	for _, c := range cases {
		if got := revealOffset(c.y, h, c.start, c.end); got != c.w {
			t.Errorf("%s: got %d, want %d", c.name, got, c.w)
		}
	}
}

func TestLolcatPadsAndColours(t *testing.T) {
	out := lolcat([]string{"ab", ""}, 5)
	if len(out) != 2 || out[0] == "ab" {
		t.Fatalf("got %q", out)
	}
	if rainbow(lolSeed) == rainbow(lolSeed+1) {
		t.Fatal("adjacent rows should differ in colour")
	}
}
