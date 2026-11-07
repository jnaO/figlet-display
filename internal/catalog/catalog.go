// Package catalog discovers installed FIGlet/TOIlet fonts and loads them
// lazily. It has no UI dependencies.
package catalog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/jnaO/figlet-font-preview/internal/figfont"
)

// SystemRoots are the package-manager font directories.
var SystemRoots = []string{
	"/opt/homebrew/share/figlet",
	"/usr/local/share/figlet",
	"/usr/share/figlet",
}

// UserDir is the per-user font directory, relative to the home directory.
const UserDir = "figlet-fonts"

// DefaultRoots returns SystemRoots followed by ~/figlet-fonts. Discover skips
// any that are missing.
func DefaultRoots() []string {
	roots := append([]string{}, SystemRoots...)
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, UserDir))
	}
	return roots
}

// State is an entry's load state.
type State int

const (
	Loading State = iota
	Ready
	Failed
)

// Entry is one discovered font.
type Entry struct {
	Name string // unique display name
	Path string // resolved real path
	Root int    // index in Catalog.Roots of the root whose walk reached the file first

	once sync.Once
	done chan struct{}
	font *figfont.Font
	err  error
}

// Catalog is an immutable, name-sorted list of fonts.
type Catalog struct {
	Roots   []string // the roots that were searched
	Entries []*Entry
	byName  map[string]*Entry
}

// Discover walks roots recursively (following symlinks) for *.flf and *.tlf
// files, deduplicating by real path. Missing roots are skipped.
func Discover(roots []string) *Catalog {
	seenFile := map[string]bool{}
	seenDir := map[string]bool{}
	var paths []string
	rootOf := map[string]int{}
	root := 0
	var walk func(dir string)
	walk = func(dir string) {
		real, err := filepath.EvalSymlinks(dir)
		if err != nil || seenDir[real] {
			return
		}
		seenDir[real] = true
		ents, err := os.ReadDir(real)
		if err != nil {
			return
		}
		for _, e := range ents {
			p := filepath.Join(real, e.Name())
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			if info.IsDir() {
				walk(p)
				continue
			}
			if !isFontFile(p) {
				continue
			}
			rp, err := filepath.EvalSymlinks(p)
			if err != nil || seenFile[rp] {
				continue
			}
			seenFile[rp] = true
			paths = append(paths, rp)
			rootOf[rp] = root
		}
	}
	for i, r := range roots {
		root = i
		walk(r)
	}
	return newCatalog(roots, paths, rootOf)
}

func isFontFile(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".flf" || ext == ".tlf"
}

func newCatalog(roots, paths []string, rootOf map[string]int) *Catalog {
	c := &Catalog{Roots: roots, byName: map[string]*Entry{}}
	for path, name := range uniqueNames(paths) {
		e := &Entry{Name: name, Path: path, Root: rootOf[path], done: make(chan struct{})}
		c.Entries = append(c.Entries, e)
		c.byName[name] = e
	}
	sort.Slice(c.Entries, func(i, j int) bool { return c.Entries[i].Name < c.Entries[j].Name })
	return c
}

// uniqueNames maps each path to its basename without extension, qualified
// with the parent directory when basenames collide, and with the full path
// if that still collides.
func uniqueNames(paths []string) map[string]string {
	base := func(p string) string { return strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)) }
	qualified := func(p string) string { return filepath.Base(filepath.Dir(p)) + "/" + base(p) }
	count := func(f func(string) string) map[string]int {
		m := map[string]int{}
		for _, p := range paths {
			m[f(p)]++
		}
		return m
	}
	bases, quals := count(base), count(qualified)
	names := make(map[string]string, len(paths))
	for _, p := range paths {
		switch {
		case bases[base(p)] == 1:
			names[p] = base(p)
		case quals[qualified(p)] == 1:
			names[p] = qualified(p)
		default:
			names[p] = strings.TrimSuffix(p, filepath.Ext(p))
		}
	}
	return names
}

// Names returns all font names in catalog order.
func (c *Catalog) Names() []string {
	out := make([]string, len(c.Entries))
	for i, e := range c.Entries {
		out[i] = e.Name
	}
	return out
}

// Get returns the entry with the given name.
func (c *Catalog) Get(name string) (*Entry, bool) {
	e, ok := c.byName[name]
	return e, ok
}

// Load parses the font on first call and caches the result. Safe for
// concurrent use; later callers block until the first parse finishes.
func (e *Entry) Load() (*figfont.Font, error) {
	e.once.Do(func() {
		e.font, e.err = figfont.ParseFile(e.Path)
		close(e.done)
	})
	return e.font, e.err
}

// State reports the entry's load state without blocking.
func (e *Entry) State() State {
	select {
	case <-e.done:
		if e.err != nil {
			return Failed
		}
		return Ready
	default:
		return Loading
	}
}

// Font returns the parsed font and error once Ready or Failed; nil, nil
// while still loading.
func (e *Entry) Font() (*figfont.Font, error) {
	if e.State() == Loading {
		return nil, nil
	}
	return e.font, e.err
}

// Preload parses every entry on a pool of workers. The returned channel is
// closed when all entries are loaded.
func (c *Catalog) Preload(workers int) <-chan struct{} {
	done := make(chan struct{})
	jobs := make(chan *Entry)
	var wg sync.WaitGroup
	for range max(workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range jobs {
				e.Load()
			}
		}()
	}
	go func() {
		for _, e := range c.Entries {
			jobs <- e
		}
		close(jobs)
		wg.Wait()
		close(done)
	}()
	return done
}
