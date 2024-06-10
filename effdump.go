// Package effdump implements the CLI tool for working with effdumps.
package effdump

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/ypsu/effdump/internal/edmain"
	"github.com/ypsu/effdump/internal/git"
	"github.com/ypsu/effdump/internal/keyvalue"
)

// Dump represesents an effdump.
type Dump edmain.Params

func (d *Dump) params() *edmain.Params {
	return (*edmain.Params)(d)
}

// New initializes a new Dump.
func New(name string) *Dump {
	d := &Dump{
		Name:   name,
		Stdout: os.Stdout,
		Env:    os.Environ(),
	}
	d.params().RegisterFlags(flag.CommandLine)
	return d
}

// Add adds a key value into the dump.
func (d *Dump) Add(key, value any) {
	d.Effects = append(d.Effects, keyvalue.KV{edmain.Stringify(key), edmain.Stringify(value)})
}

// AddMap adds each entry of the map to the dump.
// It's a standalone method due to a Go limitation around generics.
func AddMap[M ~map[K]V, K comparable, V any](d *Dump, m M) {
	d.Effects = slices.Grow(d.Effects, len(m))
	for k, v := range m {
		d.Add(k, v)
	}
}

// Run writes/diffs the effdump named `name`.
// This is meant to be overtake the main() function once the effect map is computed, this function never returns.
// Its behavior is dependent on the command line, see the package comment.
func (d *Dump) Run(ctx context.Context) {
	flag.Parse()
	d.Args = flag.Args()
	if d.FetchVersion == nil {
		d.SetVersionSystem(git.New())
	}
	if err := d.params().Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "effdump failed: %v.\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// VersionSystem fetches and resolves the source code version from the current environment.
// The returned version should be alphanumeric because it's going to be used as filenames.
type VersionSystem interface {
	Fetch(context.Context) (version string, clean bool, err error)
	Resolve(ctx context.Context, ref string) (version string, err error)
}

// SetVersionSystem overrides the version control system effdump uses.
// The default is git if this function isn't called.
func (d *Dump) SetVersionSystem(vs VersionSystem) {
	d.FetchVersion = vs.Fetch
	d.ResolveVersion = vs.Resolve
}
