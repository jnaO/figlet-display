package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	font, err := os.ReadFile("../figfont/testdata/tiny.flf")
	if err != nil {
		t.Fatal(err)
	}
	write := func(rel string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, font, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("store/alpha.flf")
	write("store/beta.TLF")
	write("store/readme.txt")
	write("store/ctrl.flc")
	write("a/dup.flf")
	write("b/dup.flf")
	if err := os.Symlink(filepath.Join(root, "store"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "store/alpha.flf"), filepath.Join(root, "alias.flf")); err != nil {
		t.Fatal(err)
	}

	c := Discover([]string{root, filepath.Join(root, "missing"), filepath.Join(root, "linked")})
	want := []string{"a/dup", "alpha", "b/dup", "beta"}
	if got := c.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}

	e, ok := c.Get("alpha")
	if !ok || e.State() != Loading {
		t.Fatalf("alpha: ok=%v state=%v", ok, e.State())
	}
	<-c.Preload(2)
	for _, e := range c.Entries {
		if e.State() != Ready {
			t.Errorf("%s: state %v", e.Name, e.State())
		}
		if f, err := e.Font(); f == nil || err != nil {
			t.Errorf("%s: font %v err %v", e.Name, f, err)
		}
	}
}

func TestLoadFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.flf"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Discover([]string{root})
	e, _ := c.Get("bad")
	if _, err := e.Load(); err == nil || e.State() != Failed {
		t.Fatalf("err=%v state=%v", err, e.State())
	}
}

func TestInstalledFonts(t *testing.T) {
	c := Discover(SystemRoots)
	if len(c.Entries) == 0 {
		t.Skip("no installed fonts")
	}
	t.Logf("%d fonts", len(c.Entries))
	<-c.Preload(8)
	for _, e := range c.Entries {
		if _, err := e.Font(); err != nil {
			t.Errorf("%s: %v", e.Name, err)
		}
	}
	for _, n := range []string{"computer", "gothic", "roman"} {
		if _, ok := c.Get(n); ok {
			t.Errorf("%s should be qualified", n)
		}
	}
}
