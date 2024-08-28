// Package effdump implements the CLI tool for working with effdumps.
//
// effdump testing is like output or golden testing but the outputs don't have to be committed into the git repository.
// It can render the diffs both in unified and HTML format and deduplicates the individual diffs.
// Convenient CLI usage is the module's main design goal.
//
// To use it create a new dump with [New], [Dump.Add] a bunch of key/value pairs, and call [Dump.Run].
// effdump takes over the rest, it makes the package into a CLI tool.
// See
//
//   - https://github.com/ypsu/effdump/tree/main/example-markdown/README.md
//   - https://github.com/ypsu/effdump/tree/main/example-deployment/README.md
//
// for how exactly it works.
// See the testing part of https://github.com/ypsu/pkgtrim for a more realistic example.
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
// The key and value are automatically stringified.
// It stringifies structs and lists without a String() function into json.
func (d *Dump) Add(key, value any) {
	d.params.Effects = append(d.params.Effects, keyvalue.KV{edmain.Stringify(key), edmain.Stringify(value)})
}

// AddMap adds each entry of the map to the dump.
// The keys and values are stringified the same way as in [Add].
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
// Usage example:
//
//	d := effdump.New("mydump")
//	d.RegisterFlags(flag.CommandLine)
//	myflag := flag.String("custominput", "", "If specified, runs mydump with this custom input.")
//	flag.Parse()
//	if *myflag { ... }
//	...
//	d.Run(ctx)
func (d *Dump) RegisterFlags(fs *flag.FlagSet) {
	d.registeredFlags = true
	d.params.RegisterFlags(fs)
}

// Run implements the CLI interface.
// This is meant to overtake the main() function: this function never returns.
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
