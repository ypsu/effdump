// Package edmain (EffDump MAIN) implements the CLI integration of the tool.
package edmain

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/ypsu/effdump/internal/keyvalue"
	"github.com/ypsu/effdump/internal/textar"
)

// Params contains most of the I/O dependencies for the Run().
type Params struct {
	Name           string
	Effects        []keyvalue.KV
	Stdout         io.Writer
	Args           []string
	Env            []string
	FetchVersion   func(context.Context) (version string, clean bool, err error)
	ResolveVersion func(ctx context.Context, ref string) (version string, err error)

	// Flags. Must be parsed by the caller after RegisterFlags.
	Force bool
	Sepch string

	// Internal helper vars.
	tmpdir  string // the dir for storing this effdump's versions
	version string // the baseline version of the source
	clean   bool   // whether the working dir is clean
}

// RegisterFlags registers effdump's flags into a flagset.
func (p *Params) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&p.Force, "force", false, "Force a save even from unclean directory.")
	fs.StringVar(&p.Sepch, "sepch", "=", "Use this character as the entry separator in the output textar.")
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
	if !p.clean && !p.Force {
		return fmt.Errorf("edmain clean check: saving from unclean workdir not allowed unless the -force flag is set")
	}

	buf, err := Compress(p.Effects, p.Sepch[0])
	if err != nil {
		return fmt.Errorf("edmain marshal: %v", err)
	}

	if err := os.MkdirAll(p.tmpdir, 0o755); err != nil {
		return fmt.Errorf("edmain make dump dir: %v", err)
	}

	fname := filepath.Join(p.tmpdir, p.version) + ".gz"
	if err := os.WriteFile(fname, buf, 0o644); err != nil {
		return fmt.Errorf("edmain save: %v", fname)
	}
	fmt.Fprintf(p.Stdout, "effdump for %s saved to %s.\n", p.version, fname)
	return nil
}

func (p *Params) cmdDiff(_ context.Context) error {
	fname := filepath.Join(p.tmpdir, p.version) + ".gz"
	buf, err := os.ReadFile(fname)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("edmain load dump: effdump for commit %v not found, git stash and save that version first", p.version)
	}
	if err != nil {
		return fmt.Errorf("edmain load dump: %v", err)
	}
	lt, err := Uncompress(buf)
	if err != nil {
		return fmt.Errorf("edmain unmarshal dump: %v", err)
	}
	rt := p.Effects

	for len(lt) > 0 && len(rt) > 0 {
		switch {
		case len(rt) == 0 || len(lt) > 0 && lt[0].K < rt[0].K:
			fmt.Fprintf(p.Stdout, "deleted: %s\n", lt[0].K)
			lt = lt[1:]
		case len(lt) == 0 || len(rt) > 0 && lt[0].K > rt[0].K:
			fmt.Fprintf(p.Stdout, "added: %s\n", rt[0].K)
			rt = rt[1:]
		case lt[0].K == rt[0].K && lt[0].V == rt[0].V:
			lt, rt = lt[1:], rt[1:]
		default:
			fmt.Fprintf(p.Stdout, "diff: %s\n", lt[0].K)
			lt, rt = lt[1:], rt[1:]
		}
	}
	return nil
}

// Run runs effdump's main CLI logic.
func (p *Params) Run(ctx context.Context) error {
	var err error
	if !isIdentifier(p.Name) {
		return fmt.Errorf("edmain check name: name %q is not a short alphanumeric identifier", p.Name)
	}
	if len(p.Sepch) != 1 {
		return fmt.Errorf("edmain sepch check: flag -sepch = %q, want a string of length 1", p.Sepch)
	}
	p.tmpdir = filepath.Join(os.TempDir(), fmt.Sprintf("effdump-%d-%s", os.Getuid(), p.Name))
	for _, e := range p.Env {
		if dir, ok := strings.CutPrefix(e, "EFFDUMP_DIR="); ok {
			p.tmpdir = dir
		}
	}
	p.version, p.clean, err = p.FetchVersion(ctx)
	if err != nil {
		return fmt.Errorf("edmain fetch version: %v", err)
	}
	if !isIdentifier(p.version) {
		return fmt.Errorf("edmain check version: %q is not a short alphanumeric identifier", p.version)
	}

	var subcommand string
	if len(p.Args) >= 1 {
		subcommand = p.Args[0]
	} else {
		if p.clean {
			fmt.Fprintln(p.Stdout, `NOTE: subcommand not given, picking "save" because working dir is clean.`)
			subcommand = "save"
		} else {
			fmt.Fprintln(p.Stdout, `NOTE: subcommand not given, picking "diff" because working dir is unclean.`)
			subcommand = "diff"
		}
	}

	slices.SortFunc(p.Effects, func(a, b keyvalue.KV) int { return cmp.Compare(a.K, b.K) })

	switch subcommand {
	case "diff":
		return p.cmdDiff(ctx)
	case "print":
		fmt.Fprintln(p.Stdout, textar.Format(p.Effects, p.Sepch[0]))
	case "save":
		return p.cmdSave(ctx)
	default:
		return fmt.Errorf("edmain run subcommand: subcommand %q not found", subcommand)
	}
	return nil
}
