package fontclass

import (
	"math"
	"sort"

	"github.com/jnaO/figlet-font-preview/internal/catalog"
)

// Input is one font for New: its catalog name, root index and profile.
type Input struct {
	Name    string
	Root    int
	Profile Profile
}

// Index answers duplicate, facet and similarity questions over a set of
// profiled fonts. Fonts that failed to parse are simply absent.
type Index struct {
	names    []string // profiled names, sorted
	profiles map[string]Profile
	rep      map[string]string   // name → its duplicate group's representative
	members  map[string][]string // representative → whole group, representative first
}

// Build profiles every loadable font in the catalog (loading as needed).
func Build(c *catalog.Catalog) *Index {
	var in []Input
	for _, e := range c.Entries {
		if f, err := e.Load(); err == nil {
			in = append(in, Input{Name: e.Name, Root: e.Root, Profile: Analyze(f)})
		}
	}
	return New(in)
}

// New builds an index from profiles. A duplicate group's representative is
// the member with the lowest Root, then the first by name.
func New(in []Input) *Index {
	ix := &Index{profiles: map[string]Profile{}, rep: map[string]string{}, members: map[string][]string{}}
	sorted := append([]Input(nil), in...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Root != sorted[j].Root {
			return sorted[i].Root < sorted[j].Root
		}
		return sorted[i].Name < sorted[j].Name
	})
	byKey := map[[32]byte]string{}
	for _, f := range sorted {
		ix.profiles[f.Name] = f.Profile
		ix.names = append(ix.names, f.Name)
		r, ok := byKey[f.Profile.Key]
		if !ok {
			r = f.Name
			byKey[f.Profile.Key] = r
		}
		ix.rep[f.Name] = r
		ix.members[r] = append(ix.members[r], f.Name)
	}
	sort.Strings(ix.names)
	return ix
}

// Profile returns the profile of a font, if it was profiled.
func (ix *Index) Profile(name string) (Profile, bool) {
	p, ok := ix.profiles[name]
	return p, ok
}

// Representative returns the font shown for name's duplicate group; an
// unprofiled font represents itself.
func (ix *Index) Representative(name string) string {
	if r, ok := ix.rep[name]; ok {
		return r
	}
	return name
}

// Duplicates returns how many other fonts share name's group when name is
// the representative, else 0.
func (ix *Index) Duplicates(name string) int {
	return max(len(ix.members[name])-1, 0)
}

// Groups returns every duplicate group with more than one member, each
// sorted by name, ordered by first member.
func (ix *Index) Groups() [][]string {
	var out [][]string
	for _, m := range ix.members {
		if len(m) > 1 {
			g := append([]string(nil), m...)
			sort.Strings(g)
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// Value returns name's value index on axis a.
func (ix *Index) Value(name string, a Axis) (int, bool) {
	p, ok := ix.profiles[name]
	return p.Values[a], ok
}

// Similar returns every other profiled font ordered by fingerprint distance
// to name, ties broken by name. Exact duplicates come first at distance 0.
func (ix *Index) Similar(name string) []string {
	p, ok := ix.profiles[name]
	if !ok {
		return nil
	}
	type cand struct {
		name string
		d    float64
	}
	cands := make([]cand, 0, len(ix.names))
	for _, n := range ix.names {
		if n != name {
			cands = append(cands, cand{n, distance(p.Fingerprint, ix.profiles[n].Fingerprint)})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].d < cands[j].d })
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.name
	}
	return out
}

func distance(a, b []float64) float64 {
	s := 0.0
	for i := range min(len(a), len(b)) {
		d := a[i] - b[i]
		s += d * d
	}
	return math.Sqrt(s)
}
