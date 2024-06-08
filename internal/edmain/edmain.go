// Package edmain (EffDump MAIN) implements the CLI integration of the tool.
package edmain

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/ypsu/effdump/internal/effect"
)

// Params contains most of the I/O dependencies for the Run().
type Params struct {
	Name           string
	Effects        []effect.Effect
	Stdout         io.Writer
	Flagset        *flag.FlagSet
	Env            []string
	FetchVersion   func(context.Context) (version string, clean bool, err error)
	ResolveVersion func(ctx context.Context, ref string) (version string, err error)
}

// Run runs effdump's main CLI logic.
func (p *Params) Run() error {
	if !p.Flagset.Parsed() {
		return fmt.Errorf("edmain: flagset not parsed")
	}

	slices.SortFunc(p.Effects, func(a, b effect.Effect) int { return cmp.Compare(a.Key, b.Key) })
	for _, e := range p.Effects {
		fmt.Fprintf(p.Stdout, "%s/%s: %q\n", p.Name, e.Key, e.Value)
	}
	return nil
}
