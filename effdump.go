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

// Run writes/diffs the effdump named `name`.
// This is meant to be overtake the main() function once the effect map is computed.
// Its behavior is dependent on the command line, see the package comment.
func Run[M ~map[K]V, K comparable, V any](name string, effects M) {
	es := make(marshal.Entries, 0, len(effects))
	for k, v := range effects {
		es = append(es, marshal.Entry{fmt.Sprint(k), fmt.Sprint(v)})
	}
	slices.SortFunc(es, func(a, b marshal.Entry) int { return cmp.Compare(a.Key, b.Key) })
	run(name, es)
}
