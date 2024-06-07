// Package effdump implements the CLI tool for working with effdumps.
package effdump

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/ypsu/effdump/internal/marshal"
)

// Overrideable for testing purposes.
var run func(name string, es marshal.Entries)

func stringify(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	if bs, ok := v.([]byte); ok {
		return string(bs)
	}
	return fmt.Sprint(v)
}

// Dump represesents an effdump.
type Dump struct {
	es marshal.Entries
}

// Add adds a key value into the dump.
func (d *Dump) Add(key, value any) {
	d.es = append(d.es, marshal.Entry{stringify(key), stringify(value)})
}

// AddMap adds each entry of the map to the dump.
// It's a standalone method due to a Go limitation around generics.
func AddMap[M ~map[K]V, K comparable, V any](d *Dump, m M) {
	d.es = slices.Grow(d.es, len(m))
	for k, v := range m {
		d.Add(k, v)
	}
}

// Run writes/diffs the effdump named `name`.
// This is meant to be overtake the main() function once the effect map is computed.
// Its behavior is dependent on the command line, see the package comment.
func (d *Dump) Run(name string) {
	slices.SortFunc(d.es, func(a, b marshal.Entry) int { return cmp.Compare(a.Key, b.Key) })
	run(name, d.es)
}
