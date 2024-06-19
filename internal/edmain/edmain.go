// Package edmain (EffDump MAIN) implements the CLI integration of the tool.
package edmain

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/ypsu/effdump/internal/andiff"
	"github.com/ypsu/effdump/internal/fmtdiff"
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
	tmpdir  string         // the dir for storing this effdump's versions
	version string         // the baseline version of the source
	clean   bool           // whether the working dir is clean
	filter  *regexp.Regexp // the entries to print or diff
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
		return fmt.Errorf("edmain/clean check: saving from unclean workdir not allowed unless the -force flag is set")
	}

	buf, err := Compress(p.Effects, p.Sepch[0])
	if err != nil {
		return fmt.Errorf("edmain/marshal: %v", err)
	}

	if err := os.MkdirAll(p.tmpdir, 0o755); err != nil {
		return fmt.Errorf("edmain/make dump dir: %v", err)
	}

	fname := filepath.Join(p.tmpdir, p.version) + ".gz"
	if err := os.WriteFile(fname, buf, 0o644); err != nil {
		return fmt.Errorf("edmain/save: %v", fname)
	}
	fmt.Fprintf(p.Stdout, "effdump for %s saved to %s.\n", p.version, fname)
	return nil
}

func (p *Params) cmdDiff(_ context.Context) error {
	fname := filepath.Join(p.tmpdir, p.version) + ".gz"
	buf, err := os.ReadFile(fname)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("edmain/load dump: effdump for commit %v not found, git stash and save that version first", p.version)
	}
	if err != nil {
		return fmt.Errorf("edmain/load dump: %v", err)
	}
	lt, err := Uncompress(buf)
	if err != nil {
		return fmt.Errorf("edmain/unmarshal dump: %v", err)
	}
	lt = slices.DeleteFunc(lt, func(kv keyvalue.KV) bool { return !p.filter.MatchString(kv.K) })
	rt := p.Effects
	kvs := make([]keyvalue.KV, 0, 16)

	for len(lt) > 0 && len(rt) > 0 {
		switch {
		case len(rt) == 0 || len(lt) > 0 && lt[0].K < rt[0].K:
			d := andiff.Compute(lt[0].V, "")
			kvs = append(kvs, keyvalue.KV{lt[0].K + " (deleted)", fmtdiff.Unified(d)})
			lt = lt[1:]
		case len(lt) == 0 || len(rt) > 0 && lt[0].K > rt[0].K:
			d := andiff.Compute("", rt[0].V)
			kvs = append(kvs, keyvalue.KV{rt[0].K + " (added)", fmtdiff.Unified(d)})
			rt = rt[1:]
		case lt[0].K == rt[0].K && lt[0].V == rt[0].V:
			lt, rt = lt[1:], rt[1:]
		default:
			d := andiff.Compute(lt[0].V, rt[0].V)
			kvs = append(kvs, keyvalue.KV{lt[0].K + " (changed)", fmtdiff.Unified(d)})
			lt, rt = lt[1:], rt[1:]
		}
	}
	for i, e := range kvs {
		if e.V != "" {
			kvs[i].V = "\t" + strings.ReplaceAll(e.V, "\n", "\n\t")
		}
	}
	fmt.Fprintln(p.Stdout, textar.Format(kvs, p.Sepch[0]))
	return nil
}

// Run runs effdump's main CLI logic.
func (p *Params) Run(ctx context.Context) error {
	var err error
	if !isIdentifier(p.Name) {
		return fmt.Errorf("edmain/check name: name %q is not a short alphanumeric identifier", p.Name)
	}
	if len(p.Sepch) != 1 {
		return fmt.Errorf("edmain/sepch check: flag -sepch = %q, want a string of length 1", p.Sepch)
	}
	p.tmpdir = filepath.Join(os.TempDir(), fmt.Sprintf("effdump-%d-%s", os.Getuid(), p.Name))
	for _, e := range p.Env {
		if dir, ok := strings.CutPrefix(e, "EFFDUMP_DIR="); ok {
			p.tmpdir = dir
		}
	}
	p.version, p.clean, err = p.FetchVersion(ctx)
	if err != nil {
		return fmt.Errorf("edmain/fetch version: %v", err)
	}
	if !isIdentifier(p.version) {
		return fmt.Errorf("edmain/check version: %q is not a short alphanumeric identifier", p.version)
	}

	var subcommand string
	var args []string
	if len(p.Args) >= 1 {
		subcommand, args = p.Args[0], p.Args[1:]
	} else {
		if p.clean {
			fmt.Fprintln(p.Stdout, `NOTE: subcommand not given, picking "save" because working dir is clean.`)
			subcommand = "save"
		} else {
			fmt.Fprintln(p.Stdout, `NOTE: subcommand not given, picking "diff" because working dir is unclean.`)
			subcommand = "diff"
		}
	}
	if subcommand == "save" && len(args) >= 1 {
		return fmt.Errorf("edmain/got %d positional arguments for save, want 0", len(args))
	}
	p.filter = MakeRE(args...)

	slices.SortFunc(p.Effects, func(a, b keyvalue.KV) int { return cmp.Compare(a.K, b.K) })
	p.Effects = slices.DeleteFunc(p.Effects, func(kv keyvalue.KV) bool { return !p.filter.MatchString(kv.K) })
	for i := 1; i < len(p.Effects); i++ {
		if p.Effects[i].K == p.Effects[i-1].K {
			return fmt.Errorf("edmain/unique check: key %q duplicated", p.Effects[i].K)
		}
	}

	switch subcommand {
	case "diff":
		return p.cmdDiff(ctx)
	case "hash":
		if len(args) > 0 {
			return fmt.Errorf("edmain/hash: got %d args, want 0", len(args))
		}
		fmt.Fprintf(p.Stdout, "%016x\n", Hash(p.Effects))
		return nil
	case "print":
		kvs := slices.Clone(p.Effects)
		for i, e := range kvs {
			if e.V != "" {
				kvs[i].V = "\t" + strings.ReplaceAll(e.V, "\n", "\n\t")
			}
		}
		fmt.Fprintln(p.Stdout, textar.Format(kvs, p.Sepch[0]))
		return nil
	case "printraw":
		if len(args) != 1 {
			return fmt.Errorf("edmain/printraw: got %d args, want 1", len(args))
		}
		for _, e := range p.Effects {
			if e.K == args[0] {
				fmt.Fprint(p.Stdout, e.V)
				return nil
			}
		}
		return fmt.Errorf("edmain/printraw: key %q not found", args[0])
	case "save":
		return p.cmdSave(ctx)
	default:
		return fmt.Errorf("edmain/run subcommand: subcommand %q not found", subcommand)
	}
}

// MakeRE makes a single regex from a set of globs.
func MakeRE(globs ...string) *regexp.Regexp {
	if len(globs) == 0 {
		return regexp.MustCompile("")
	}
	expr := &strings.Builder{}
	expr.WriteString("^(")
	for i, glob := range globs {
		if i != 0 {
			expr.WriteByte('|')
		}
		parts := strings.Split(glob, "*")
		for i, part := range parts {
			parts[i] = regexp.QuoteMeta(part)
		}
		expr.WriteString(strings.Join(parts, ".*"))
	}
	expr.WriteString(")$")
	return regexp.MustCompile(expr.String())
}

// Hash hashes a keyvalue slice.
func Hash(kvs []keyvalue.KV) uint64 {
	h := fnv.New64()
	for _, kv := range kvs {
		h.Write([]byte(kv.K))
		h.Write([]byte(kv.V))
	}
	return h.Sum64()
}
