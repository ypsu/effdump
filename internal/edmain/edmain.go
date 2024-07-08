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
	"math/rand"
	"net"
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
	Color      string
	Force      bool
	OutputFile string
	Sepch      string
	Subkey     string
	Watch      bool

	// Internal helper vars.
	tmpdir     string         // the dir for storing this effdump's versions
	version    string         // the baseline version of the source
	clean      bool           // whether the working dir is clean
	filter     *regexp.Regexp // the entries to print or diff
	watcherpid string         // parent -watch process PID, if one is running
}

// Usage prints a help message to p.Stdout.
func (p *Params) Usage() {
	p.Stdout.Write([]byte(`effdump: generate and diff an effect dump.

Most important subcommands to know about:

- diff: Print an unified diff between HEAD dump and the current version. Takes a list of key globs for filtering.
- save: Save the current version of the dump to the temp dir.
- webdiff: Serve the HTML formatted diff between HEAD dump and the current version.

All subcommands:

- clear: Delete this effdump's cache: all previously stored dumps and html reports in its temp dir.
- diff: Print an unified diff between HEAD dump and the current version. Takes a list of key globs for filtering.
- help: This usage string.
- hash: Prints the hash of the dump. The hash includes the key names too.
- htmldiff: Generate a HTML formatted diff between HEAD dump and the current version.
- keys: Print the list of keys the dump has.
- print: Print the dump to stdout. Takes a list of key globs for filtering.
- printraw: Print one effect to stdout without any decoration. Needs one argument for the key.
- save: Save the current version of the dump to the temp dir.
- webdiff: Serve the HTML formatted diff between HEAD dump and the current version.
- webprintraw: Same as printraw but serves it over HTTP.

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
	fs.StringVar(&p.Color, "color", "auto", "Whether to colorize the output. Valid values: auto|yes|no.")
	fs.BoolVar(&p.Force, "force", false, "Force a save even from unclean directory.")
	fs.StringVar(&p.OutputFile, "o", "", "Override the output file for htmldiff and htmlprint. Use - to write to stdout.")
	fs.StringVar(&p.Sepch, "sepch", "=", "Use this character as the entry separator in the output textar.")
	fs.StringVar(&p.Subkey, "subkey", "",
		"Parse each value as a textar, pick subkey's value, and then operate on that section only.\n"+
			"Especially useful for printraw to print a portion of the result.")
	fs.BoolVar(&p.Watch, "watch", false, "If set then continuously re-run the command on any file change under the current directory. Linux only.")
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
func (p *Params) diff() ([]fmtdiff.Bucket, error) {
	fname := filepath.Join(p.tmpdir, p.version) + ".gz"
	buf, err := os.ReadFile(fname)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("edmain/load dump: effdump for commit %v not found, git stash and save that version first", p.version)
	}
	if err != nil {
		return nil, fmt.Errorf("edmain/load dump: %v", err)
	}
	lt, err := Uncompress(buf)
	if err != nil {
		return nil, fmt.Errorf("edmain/unmarshal dump: %v", err)
	}
	for i := 1; i < len(lt); i++ {
		if lt[i].K <= lt[i-1].K {
			return nil, fmt.Errorf("edmain/sort check of %s: %dth key not in order (corrupted? re-save the version)", p.version, i)
		}
	}
	lt = slices.DeleteFunc(lt, func(kv keyvalue.KV) bool { return !p.filter.MatchString(kv.K) })
	p.subkeyize(lt)
	rt := p.Effects

	n, e, buckets, hash2idx := 0, fmtdiff.Entry{}, []fmtdiff.Bucket{}, map[uint64]int{}
	for len(lt) > 0 || len(rt) > 0 {
		switch {
		case len(rt) == 0 || len(lt) > 0 && lt[0].K < rt[0].K:
			e = fmtdiff.Entry{lt[0].K, "deleted", andiff.Compute(lt[0].V, "")}
			lt, n = lt[1:], n+1
		case len(lt) == 0 || len(rt) > 0 && lt[0].K > rt[0].K:
			e = fmtdiff.Entry{rt[0].K, "added", andiff.Compute("", rt[0].V)}
			rt, n = rt[1:], n+1
		case lt[0].K == rt[0].K && lt[0].V == rt[0].V:
			lt, rt = lt[1:], rt[1:]
			continue
		default:
			e = fmtdiff.Entry{lt[0].K, "changed", andiff.Compute(lt[0].V, rt[0].V)}
			lt, rt, n = lt[1:], rt[1:], n+1
		}
		idx, exists := hash2idx[e.Diff.Hash]
		if !exists {
			idx, hash2idx[e.Diff.Hash], buckets = len(buckets), len(buckets), append(buckets, fmtdiff.Bucket{Hash: e.Diff.Hash})
		}
		buckets[idx].Entries = append(buckets[idx].Entries, e)
	}
	slices.SortFunc(buckets, func(a, b fmtdiff.Bucket) int { return cmp.Compare(a.Entries[0].Name, b.Entries[0].Name) })
	return buckets, nil
}

func (p *Params) subkeyize(kvs []keyvalue.KV) {
	if p.Subkey == "" {
		return
	}
	subkvs := make([]keyvalue.KV, 0, 4)
	for i, e := range kvs {
		subkvs, p.Effects[i].V = textar.Parse(subkvs, e.V), ""
		for _, kv := range subkvs {
			if kv.K == p.Subkey {
				kvs[i].V = kv.V
				break
			}
		}
	}
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
		if pid, ok := strings.CutPrefix(e, "EFFDUMP_WATCHERPID="); ok {
			p.watcherpid = pid
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
	p.subkeyize(p.Effects)

	if p.Watch && p.watcherpid == "" {
		return p.watch(ctx)
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
		buckets, err := p.diff()
		if err != nil {
			return fmt.Errorf("edmain/diff: %v", err)
		}
		if len(buckets) == 0 {
			fmt.Fprintln(p.Stdout, "NOTE: No diffs.")
			return nil
		}
		_, err = io.WriteString(p.Stdout, fmtdiff.UnifiedBuckets(buckets, p.Sepch[0]))
		if err != nil {
			return fmt.Errorf("edmain/write unified diff: %v", err)
		}
		return nil
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
		buckets, err := p.diff()
		if err != nil {
			return fmt.Errorf("edmain/htmldiff: %v", err)
		}
		if len(buckets) == 0 {
			fmt.Fprintln(p.Stdout, "NOTE: No diffs.")
			return nil
		}
		html := fmtdiff.HTMLBuckets(buckets)
		if p.OutputFile == "-" {
			_, err := io.WriteString(p.Stdout, html)
			if err != nil {
				return fmt.Errorf("edmain/htmldiff to stdout: %v", err)
			}
			return nil
		}
		err = os.WriteFile(p.OutputFile, []byte(html), 0644)
		if err != nil {
			return fmt.Errorf("edmain/write htmldiff file: %v", err)
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
	case "webdiff":
		buckets, err := p.diff()
		if err != nil {
			return fmt.Errorf("edmain/webdiff: %v", err)
		}
		if len(buckets) == 0 {
			fmt.Fprintln(p.Stdout, "NOTE: No diffs.")
			return nil
		}
		html := fmtdiff.HTMLBuckets(buckets)
		return p.serve(ctx, html)
	case "webprintraw":
		if len(args) != 1 {
			return fmt.Errorf("edmain/printraw: got %d args, want 1", len(args))
		}
		for _, e := range p.Effects {
			if e.K == args[0] {
				return p.serve(ctx, e.V)
			}
		}
		return fmt.Errorf("edmain/printraw: key %q not found", args[0])
	default:
		return fmt.Errorf("edmain/run subcommand: subcommand %q not found", subcommand)
	}
}

func (p *Params) serve(_ context.Context, s string) error {
	t, content := time.Now(), strings.NewReader(s)
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.ServeContent(w, req, "", t, content)
	})
	listener, err := net.Listen("tcp", p.Address)
	if err != nil {
		return fmt.Errorf("edmain/listen on %s: %v", p.Address, err)
	}
	fmt.Fprintf(os.Stderr, "Serving HTTP on %s.\n", p.Address)
	p.notifyWatcher() // Tell watcher (if any) that the output is ready.
	return http.Serve(listener, handler)
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
