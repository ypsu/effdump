// Package effdump implements the CLI tool for working with effdumps.
package effdump

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/ypsu/effdump/internal/edmain"
	"github.com/ypsu/effdump/internal/effect"
)

// Dump represesents an effdump.
type Dump struct {
	effects []effect.Effect
}

// New initializes a new Dump.
func New() *Dump {
	return &Dump{}
}

// Add adds a key value into the dump.
func (d *Dump) Add(key, value any) {
	d.effects = append(d.effects, effect.Effect{effect.Stringify(key), effect.Stringify(value)})
}

// AddMap adds each entry of the map to the dump.
// It's a standalone method due to a Go limitation around generics.
func AddMap[M ~map[K]V, K comparable, V any](d *Dump, m M) {
	d.effects = slices.Grow(d.effects, len(m))
	for k, v := range m {
		d.Add(k, v)
	}
}

// Run writes/diffs the effdump named `name`.
// This is meant to be overtake the main() function once the effect map is computed, this function never returns.
// Its behavior is dependent on the command line, see the package comment.
func (d *Dump) Run(name string) {
	err := (&edmain.Params{
		Name:    name,
		Effects: d.effects,
		Stdout:  os.Stdout,
		Flagset: flag.CommandLine,
		Env:     os.Environ(),
	}).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
