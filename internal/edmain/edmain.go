// Package edmain (EffDump MAIN) implements the CLI integration of the tool.
package edmain

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"unicode"

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

	cachedir string // the user's cache directory
	version  string // the baseline version of the source
	clean    bool   // whether the working dir is clean
}

func isIdentifier(v string) bool {
	if len(v) == 0 || len(v) > 64 {
		return false
	}
	for _, ch := range v {
		if !unicode.IsDigit(ch) && !unicode.IsLetter(ch) {
			return false
		}
	}
	return true
}

func (p *Params) cmdSave(_ context.Context) error {
	// TODO: allow disabling this with a flag.
	if !p.clean {
		return fmt.Errorf("edmain clean check: saving from unclean workdir not allowed unless the -force flag is set")
	}

	buf, err := effect.Marshal(p.Effects)
	if err != nil {
		return fmt.Errorf("edmain marshal: %v", err)
	}

	dir := filepath.Join(p.cachedir, "effdump", p.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("edmain make dump dir: %v", err)
	}

	fname := filepath.Join(dir, p.version)
	if err := os.WriteFile(fname, buf, 0o644); err != nil {
		return fmt.Errorf("edmain save: %v", fname)
	}
	fmt.Fprintf(p.Stdout, "effdump for %s saved to %s.\n", p.version, fname)
	return nil
}

// Run runs effdump's main CLI logic.
func (p *Params) Run(ctx context.Context) error {
	var err error
	if !p.Flagset.Parsed() {
		return fmt.Errorf("edmain check flagset: flagset not parsed")
	}
	p.cachedir, err = os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("edmain get cachedir: %v", err)
	}
	p.version, p.clean, err = p.FetchVersion(ctx)
	if err != nil {
		return fmt.Errorf("edmain fetch version: %v", err)
	}
	if !isIdentifier(p.version) {
		return fmt.Errorf("edmain check version: %q is not a short alphanumeric identifier", p.version)
	}

	subcommand := p.Flagset.Arg(0)
	if subcommand == "" {
		if p.clean {
			fmt.Fprintln(p.Stdout, `NOTE: subcommand not given, picking "save" because working dir is clean.`)
			subcommand = "save"
		} else {
			fmt.Fprintln(p.Stdout, `NOTE: subcommand not given, picking "diff" because working dir is unclean.`)
			subcommand = "diff"
		}
	}

	slices.SortFunc(p.Effects, func(a, b effect.Effect) int { return cmp.Compare(a.Key, b.Key) })

	switch subcommand {
	case "save":
		return p.cmdSave(ctx)
	case "print":
		for _, e := range p.Effects {
			fmt.Fprintf(p.Stdout, "%s/%s: %q\n", p.Name, e.Key, e.Value)
		}
	default:
		return fmt.Errorf("edmain run subcommand: subcommand %q not found", subcommand)
	}
	return nil
}
