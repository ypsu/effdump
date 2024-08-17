// Package effdump implements the CLI tool for working with effdumps.
package effdump

import (
	"bufio"
	"cmp"
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/ypsu/effdump/internal/edmain"
	"github.com/ypsu/effdump/internal/git"
	"github.com/ypsu/effdump/internal/keyvalue"
)

// Dump represesents an effdump.
type Dump struct {
	params          edmain.Params
	registeredFlags bool
}

// New initializes a new Dump.
func New(name string) *Dump {
	d := &Dump{params: edmain.Params{
		Name:   name,
		Env:    os.Environ(),
		Stdout: os.Stderr, // temporary for error reporting
	}}
	return d
}

// Add adds a key value into the dump.
func (d *Dump) Add(key, value any) {
	d.params.Effects = append(d.params.Effects, keyvalue.KV{edmain.Stringify(key), edmain.Stringify(value)})
}

// AddMap adds each entry of the map to the dump.
// It's a standalone method due to a Go limitation around generics.
func AddMap[M ~map[K]V, K comparable, V any](d *Dump, m M) {
	d.params.Effects = slices.Grow(d.params.Effects, len(m))
	for k, v := range m {
		d.Add(k, v)
	}
}

// Hash hashes the values in the dump.
// Returns the same value as the hash subcommand.
// Returns 0 if there are duplicated keys in the dump.
func (d *Dump) Hash() uint64 {
	slices.SortFunc(d.params.Effects, func(a, b keyvalue.KV) int { return cmp.Compare(a.K, b.K) })
	for i := 1; i < len(d.params.Effects); i++ {
		if d.params.Effects[i].K == d.params.Effects[i-1].K {
			return 0
		}
	}
	return edmain.Hash(d.params.Effects)
}

// RegisterFlags registers effdump's flags into a flagset.
// If not called, flags are autoregistered into flag.CommandLine in Run().
func (d *Dump) RegisterFlags(fs *flag.FlagSet) {
	d.registeredFlags = true
	d.params.RegisterFlags(fs)
}

// Run writes/diffs the effdump named `name`.
// This is meant to be overtake the main() function once the effect map is computed, this function never returns.
// Its behavior is dependent on the command line, see the package comment.
func (d *Dump) Run(ctx context.Context) {
	if !d.registeredFlags {
		d.RegisterFlags(flag.CommandLine)
	}
	flag.Parse()

	// Check for bad flag usage.
	positionalPart := false
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			if positionalPart {
				fmt.Fprintf(os.Stderr, "ERROR: %q looks like a flag as a positional argument; put flags before the subcommand, use the -flag=value syntax, and use `--` on its own to separate flags from args.\n", arg)
				os.Exit(1)
			}
		} else {
			positionalPart = true
		}
	}

	d.params.Args = flag.Args()
	if d.params.VSHasChanges == nil {
		d.SetVersionSystem(git.New())
	}

	out := bufio.NewWriter(os.Stdout)
	d.params.Stdout = out
	err := d.params.Run(ctx)
	out.Flush()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v.\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// VersionSystem resolves source code versions from the current environment.
type VersionSystem interface {
	// HasChanges tells effdump whether the current directory has changes compared the HEAD revision.
	// effdump uses this to determine the default action when used with no-args mode.
	HasChanges(context.Context) (dirty bool, err error)

	// Resolve references like "HEAD" and "HEAD^" to a version.
	// effdump passes empty revision to look up the current HEAD.
	// The returned version should be alphanumeric because it's going to be used as filenames.
	Resolve(ctx context.Context, revision string) (version string, err error)
}

// SetVersionSystem overrides the version control system effdump uses.
// The default is git if this function isn't called.
func (d *Dump) SetVersionSystem(vs VersionSystem) {
	d.params.VSHasChanges = vs.HasChanges
	d.params.VSResolve = vs.Resolve
}
