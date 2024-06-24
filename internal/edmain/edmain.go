// Package edmain (EffDump MAIN) implements the CLI integration of the tool.
package edmain

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/ypsu/effdump/internal/andiff"
	"github.com/ypsu/effdump/internal/edbg"
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
	Flagset        *flag.FlagSet // for Usage().
	FetchVersion   func(context.Context) (version string, clean bool, err error)
	ResolveVersion func(ctx context.Context, ref string) (version string, err error)

	// Flags. Must be parsed by the caller after RegisterFlags.
	Address    string
	Force      bool
	OutputFile string
	Sepch      string

	// Internal helper vars.
	tmpdir  string         // the dir for storing this effdump's versions
	version string         // the baseline version of the source
	clean   bool           // whether the working dir is clean
	filter  *regexp.Regexp // the entries to print or diff
}

// Usage prints a help message to p.Stdout.
func (p *Params) Usage() {
	p.Stdout.Write([]byte(`effdump: generate and diff an effect dump.

Subcommands:

- clear: Delete this effdump's cache: all previously stored dumps and html reports in its temp dir.
- diff: Print an unified diff between HEAD dump and the current version. Takes a list of key globs for filtering.
- help: This usage string.
- hash: Prints the hash of the dump. The hash includes the key names too.
- htmldiff: Generate a HTML formatted diff between HEAD dump and the current version.
- keys: Print the list of keys the dump has.
- print: Print the dump to stdout. Takes a list of key globs for filtering.
- printraw: Print one effect to stdout without any decoration. Needs one argument for the key.
- save: Save the current version of the dump to the temp dir.
- web: Serve the HTML formatted diff between HEAD dump and the current version.

Key globs: * is replaced with arbitrary number of characters. "hello" matches the glob "*o*".

Flags:

`))
	p.Flagset.SetOutput(p.Stdout)
	p.Flagset.PrintDefaults()
}

// RegisterFlags registers effdump's flags into a flagset.
func (p *Params) RegisterFlags(fs *flag.FlagSet) {
	p.Flagset = fs
	fs.Usage = p.Usage
	fs.StringVar(&p.Address, "address", ":8080", "The address to serve webdiff on.")
	fs.BoolVar(&p.Force, "force", false, "Force a save even from unclean directory.")
	fs.StringVar(&p.OutputFile, "o", "", "Override the output file for htmldiff and htmlprint. Use - to write to stdout.")
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
	hash := Hash(p.Effects)

	if err := os.MkdirAll(p.tmpdir, 0o755); err != nil {
		return fmt.Errorf("edmain/make dump dir: %v", err)
	}

	fname := filepath.Join(p.tmpdir, p.version) + ".gz"
	if f, err := os.Open(fname); err == nil {
		gotHash := PeekHash(f)
		f.Close()
		if gotHash == hash {
			fmt.Fprintf(p.Stdout, "NOTE: skipped writing %s because it already exists and looks the same.\n", fname)
			return nil
		}
	}

	buf, err := Compress(p.Effects, p.Sepch[0], hash)
	if err != nil {
		return fmt.Errorf("edmain/marshal: %v", err)
	}
	if err := os.WriteFile(fname, buf, 0o644); err != nil {
		return fmt.Errorf("edmain/save: %v", fname)
	}
	fmt.Fprintf(p.Stdout, "effdump for %s saved to %s.\n", p.version, fname)
	return nil
}

// diff diffs the current version against the baseline and records the diffs.
// Returns 0 if both sides are the same.
func (p *Params) diff(record func(string, andiff.Diff)) (int, error) {
	fname := filepath.Join(p.tmpdir, p.version) + ".gz"
	buf, err := os.ReadFile(fname)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("edmain/load dump: effdump for commit %v not found, git stash and save that version first", p.version)
	}
	if err != nil {
		return 0, fmt.Errorf("edmain/load dump: %v", err)
	}
	lt, err := Uncompress(buf)
	if err != nil {
		return 0, fmt.Errorf("edmain/unmarshal dump: %v", err)
	}
	for i := 1; i < len(lt); i++ {
		if lt[i].K <= lt[i-1].K {
			return 0, fmt.Errorf("edmain/sort check of %s: %dth key not in order (corrupted? re-save the version)", p.version, i)
		}
	}
	lt = slices.DeleteFunc(lt, func(kv keyvalue.KV) bool { return !p.filter.MatchString(kv.K) })
	rt := p.Effects

	n := 0
	for len(lt) > 0 && len(rt) > 0 {
		switch {
		case len(rt) == 0 || len(lt) > 0 && lt[0].K < rt[0].K:
			record(lt[0].K+" (deleted)", andiff.Compute(lt[0].V, ""))
			lt, n = lt[1:], n+1
		case len(lt) == 0 || len(rt) > 0 && lt[0].K > rt[0].K:
			record(rt[0].K+" (added)", andiff.Compute("", rt[0].V))
			rt, n = rt[1:], n+1
		case lt[0].K == rt[0].K && lt[0].V == rt[0].V:
			lt, rt = lt[1:], rt[1:]
		default:
			record(lt[0].K+" (changed)", andiff.Compute(lt[0].V, rt[0].V))
			lt, rt, n = lt[1:], rt[1:], n+1
		}
	}
	return n, nil
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
	if p.OutputFile == "" {
		const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		now := time.Now()
		for {
			p.OutputFile = filepath.Join(p.tmpdir, fmt.Sprintf("%d%c%04d.html", now.Year()%100, alpha[min(now.YearDay()/7, 51)], rand.Intn(1e4)))
			if _, err := os.Stat(p.OutputFile); err != nil {
				break
			}
		}
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
	case "clear":
		if len(args) > 0 {
			return fmt.Errorf("edmain/clear: got %d args, want 0", len(args))
		}
		var deletedFiles int
		files, _ := filepath.Glob(filepath.Join(p.tmpdir, "*.gz"))
		htmlFiles, _ := filepath.Glob(filepath.Join(p.tmpdir, "*.html"))
		for _, f := range append(files, htmlFiles...) {
			if os.Remove(f) == nil { // on success
				edbg.Printf("Deleted %s.\n", filepath.Base(f))
				deletedFiles++
			}
		}
		os.Remove(p.tmpdir)
		fmt.Fprintf(p.Stdout, "Removed %d files from %s.\n", deletedFiles, p.tmpdir)
		return nil
	case "diff":
		uf := fmtdiff.NewUnifiedFormatter(p.Sepch[0])
		n, err := p.diff(uf.Add)
		if err != nil {
			return fmt.Errorf("edmain/diff: %v", err)
		}
		if n == 0 {
			fmt.Fprintln(p.Stdout, "NOTE: No diffs.")
			return nil
		}
		_, err = uf.WriteTo(p.Stdout)
		return err
	case "hash":
		if len(args) > 0 {
			return fmt.Errorf("edmain/hash: got %d args, want 0", len(args))
		}
		fmt.Fprintf(p.Stdout, "%016x\n", Hash(p.Effects))
		return nil
	case "help":
		p.Usage()
		return nil
	case "htmldiff":
		hf := fmtdiff.NewHTMLFormatter()
		n, err := p.diff(hf.Add)
		if err != nil {
			return fmt.Errorf("edmain/htmldiff: %v", err)
		}
		if n == 0 {
			fmt.Fprintln(p.Stdout, "NOTE: No diffs.")
			return nil
		}
		if p.OutputFile == "-" {
			_, err := hf.WriteTo(p.Stdout)
			if err != nil {
				return fmt.Errorf("edmain/htmldiff to stdout: %v", err)
			}
			return nil
		}
		w := bytes.NewBuffer(make([]byte, 0, 1<<16))
		hf.WriteTo(w)
		err = os.WriteFile(p.OutputFile, w.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("edmain/htmldiff: %v", err)
		}
		err = os.WriteFile(p.OutputFile, w.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("edmain/htmldiff: %v", err)
		}
		fmt.Fprintf(p.Stdout, "NOTE: Output written to %s.\n", p.OutputFile)
		return nil
	case "keys":
		for _, e := range p.Effects {
			fmt.Fprintln(p.Stdout, e.K)
		}
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
	case "web":
		hf := fmtdiff.NewHTMLFormatter()
		n, err := p.diff(hf.Add)
		if err != nil {
			return fmt.Errorf("edmain/web: %v", err)
		}
		if n == 0 {
			fmt.Fprintln(p.Stdout, "NOTE: No diffs.")
			return nil
		}
		w := &strings.Builder{}
		w.Grow(4096)
		hf.WriteTo(w)
		t, content := time.Now(), strings.NewReader(w.String())
		handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			http.ServeContent(w, req, "diff.html", t, content)
		})
		fmt.Fprintf(os.Stderr, "Serving HTML diff on %s.\n", p.Address)
		if err := http.ListenAndServe(p.Address, handler); err != nil {
			return fmt.Errorf("edmain/ListenAndServe: %v", err)
		}
		return nil
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
