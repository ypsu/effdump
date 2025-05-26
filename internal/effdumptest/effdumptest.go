// Binary effdumptest generates the effdump library's effects.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ypsu/effdump"
	"github.com/ypsu/effdump/internal/andiff"
	"github.com/ypsu/effdump/internal/edbg"
	"github.com/ypsu/effdump/internal/edmain"
	"github.com/ypsu/effdump/internal/edtextar"
	"github.com/ypsu/effdump/internal/fmtdiff"
	"github.com/ypsu/effdump/internal/keyvalue"
)

//go:embed *.textar
var testdataFS embed.FS

func addStringifyEffects(d *effdump.Dump) {
	d.Add("stringify/int", 42)
	d.Add("stringify/byte", 'a')
	d.Add("stringify/stringer", time.UnixMilli(0).UTC())
	d.Add("stringify/error", io.EOF)
	d.Add("stringify/string", "hello world")
	d.Add("stringify/multiline-string", "this\nis\na\nmultiline\nstring\n")
	d.Add("stringify/int-slice", []int{1, 2, 3, 4, 5})
	d.Add("stringify/int-string-map", map[int]string{1: "one", 2: "two", 3: "three"})
	d.Add("stringify/int-multilinestring-map", map[int]string{1: "one line", 2: "two\nlines", 3: "three\nshort\nlines\n", 4: "four\nmore\nshort\nlines"})
	d.Add("stringify/struct-list", []struct {
		I       int
		V       []string
		private int
	}{{1, []string{"a", "b"}, 7}, {2, []string{"multiline\nstring"}, 9}})
}

func testdata(fn string) string {
	buf, err := testdataFS.ReadFile(fn)
	if err != nil {
		panic(fmt.Errorf("effdumptest/read testdata: %v", err))
	}
	return string(buf)
}

func mkdump() (*effdump.Dump, error) {
	debuglog := &strings.Builder{}
	edbg.Printf = func(format string, v ...any) { fmt.Fprintf(debuglog, format, v...) }
	defer edbg.Reset()

	// Set up a tmpdir for EFFDUMP_DIR.
	tmpdir, err := os.MkdirTemp("", "effdump-tmp-")
	if err != nil {
		return nil, fmt.Errorf("effdumptest/make temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	ctx := context.Background()
	d := effdump.New("effdumptest")
	addStringifyEffects(d)

	// Test the glob parser.
	addglob := func(name string, globs ...string) {
		d.Add("globs/"+name, fmt.Sprintf("%q\n%s", globs, edmain.MakeRE(globs...)))
	}
	addglob("empty")
	addglob("static-one", "apple")
	addglob("static-two", "apple", "banana")
	addglob("special-three", "*apple", "*ba*na*na", "cherry*", "da.|?[a-z]te")

	// Test the differ.
	diffar := edtextar.Parse(nil, testdata("trickydiffs.textar"))
	for _, kv := range diffar {
		lt, rt := &strings.Builder{}, &strings.Builder{}
		name, args, _ := strings.Cut(kv.K, " ")
		for _, line := range strings.Split(kv.V, "\n") {
			if line == "" {
				line = " "
			}
			firstchar := line[0]
			switch firstchar {
			case '-':
				lt.WriteString(line[1:] + "\n")
			case '+':
				rt.WriteString(line[1:] + "\n")
			default:
				lt.WriteString(line[1:] + "\n")
				rt.WriteString(line[1:] + "\n")
			}
		}

		kvs := make([]keyvalue.KV, 0, 4)
		var rmregexp *regexp.Regexp
		for _, arg := range strings.Split(args, " ") {
			if arg == "" {
				continue
			} else if re, ok := strings.CutPrefix(arg, "-x="); ok {
				rmregexp = regexp.MustCompile(re)
			} else {
				kvs = append(kvs, keyvalue.KV{"error", "invalid arg: " + arg})
			}
		}
		diff := andiff.Compute(lt.String(), rt.String(), rmregexp)
		if args != "" {
			kvs = append(kvs, keyvalue.KV{"args", args + "\n"})
		}
		w := &strings.Builder{}
		kvs = append(kvs, keyvalue.KV{"input", kv.V})
		fmt.Fprintf(w, "Hash: 0x%016x\n", diff.Hash)
		for _, op := range diff.Ops {
			fmt.Fprintf(w, "%+v\n", op)
		}
		kvs = append(kvs, keyvalue.KV{"diff", w.String()})
		if debuglog.Len() > 0 {
			kvs = append(kvs, keyvalue.KV{"debuglog", debuglog.String()})
			debuglog.Reset()
		}
		kvs = append(kvs, keyvalue.KV{"unified", fmtdiff.Unified(diff, 3, false)})
		d.Add("diffs/"+name+".txt", edtextar.Format(kvs, '-'))
		buckets := []fmtdiff.Bucket{{Entries: []fmtdiff.Entry{{Name: "html", Diff: diff}}}}
		d.Add("diffs/"+name+".html", fmtdiff.HTMLBuckets(buckets, nil, 3))
	}

	// Set up common helpers for the CLI tests.
	group, key, desc, w, log := "", "", "", &strings.Builder{}, &strings.Builder{}
	fetchVersion, fetchDirty, fetchErr := "numsbase", false, error(nil)
	baseParams := edmain.Params{
		Name:    "testdump",
		Effects: edtextar.Parse(nil, testdata("numsbase.textar")),
		Env:     []string{"EFFDUMP_DIR=" + tmpdir},
		Stdout:  w,

		VSHasChanges: func(context.Context) (bool, error) {
			edbg.Printf("VSHasChanges() -> (%t, %v)\n", fetchDirty, fetchErr)
			return fetchDirty, fetchErr
		},

		VSResolve: func(_ context.Context, ref string) (string, error) {
			edbg.Printf("VSResolve(%q) -> (%q, %v)\n", ref, fetchVersion, fetchErr)
			return fetchVersion, fetchErr
		},
	}
	p := baseParams
	run := func(args ...string) {
		kvs := make([]keyvalue.KV, 1, 4)
		kvs[0] = keyvalue.KV{"desc", fmt.Sprintf("%s\n\nargs: testdump %q", desc, args)}
		fs := flag.NewFlagSet("effdumptest", flag.ContinueOnError)
		p.RegisterFlags(fs)
		p.Color = "no"
		if err := fs.Parse(args); err != nil {
			kvs = append(kvs, keyvalue.KV{"flagparse-error", err.Error()})
		}
		p.Args = fs.Args()
		p.Sepch = "-"
		err := p.Run(ctx)
		if w.Len() > 0 {
			stdout := strings.ReplaceAll(w.String(), tmpdir, "/tmpdir")
			kvs = append(kvs, keyvalue.KV{"stdout", stdout})
			w.Reset()
		}
		if err != nil {
			kvs = append(kvs, keyvalue.KV{"error", err.Error()})
		}
		if log.Len() > 0 {
			kvs = append(kvs, keyvalue.KV{"log", log.String()})
			log.Reset()
		}
		if debuglog.Len() > 0 {
			kvs = append(kvs, keyvalue.KV{"debuglog", debuglog.String()})
			debuglog.Reset()
		}
		d.Add(key, edtextar.Format(kvs, '~'))

		// Reset defaults for the next testcase.
		p, fetchVersion, fetchDirty = baseParams, "numsbase", false
		p.Effects = edtextar.Parse(nil, testdata("numsbase.textar"))
	}
	setdesc := func(name, description string) { key, desc = fmt.Sprintf("%s/%s", group, name), description }

	// The baseline for the following tests will be numsbase.
	numsbase := edtextar.Parse(nil, testdata("numsbase.textar"))
	gz, err := edmain.Compress(numsbase, '=', edmain.Hash(numsbase))
	if err != nil {
		return nil, fmt.Errorf("effdumptest/compress numsbase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpdir, "numsbase.gz"), gz, 0o644); err != nil {
		return nil, fmt.Errorf("effdumptest/write numsbase.gz: %v", err)
	}

	group = "cmd-help"
	setdesc("help", "Help prints the usage string.")
	run("help")

	group = "cmd-print"
	setdesc("no-args", "Printing without args should print all the effects.")
	run("print")
	setdesc("keyptr", "Printing with -keyptr should print only the target effect.")
	run("-keyptr=html.ptr", "print")
	setdesc("two-args", "Printing without args should print only the even and odd effects.")
	run("print", "even", "odd")
	setdesc("oddglob-arg", "Printing without args should print all effects starting with 'o*'.")
	run("print", "odd*")
	setdesc("glob-arg", "Printing without args should print all effects containing 'o'.")
	run("print", "*o*")
	setdesc("dup-error", "There's a duplicate entry added in this one.")
	p.Effects = append(p.Effects, keyvalue.KV{"all", "another all entry"})
	run("print")

	group = "cmd-htmlprint"
	setdesc("no-args", "Printing without args should print all the effects.")
	run("htmlprint")
	setdesc("two-args", "Printing without args should print only the even and odd effects.")
	run("htmlprint", "even", "odd")
	setdesc("oddglob-arg", "Printing without args should print all effects starting with 'o*'.")
	run("htmlprint", "odd*")
	setdesc("glob-arg", "Printing without args should print all effects containing 'o'.")
	run("htmlprint", "*o*")
	setdesc("dup-error", "There's a duplicate entry added in this one.")
	p.Effects = append(p.Effects, keyvalue.KV{"all", "another all entry"})
	run("htmlprint")

	group = "cmd-keys"
	setdesc("no-args", "Printing without args should print all the keys.")
	run("keys")
	group = "cmd-keys"
	setdesc("globs", "Printing with args should print the matching keys.")
	run("keys", "*o*")

	group = "cmd-printraw"
	setdesc("no-args", "printraw expects one argument exactly.")
	run("printraw")
	setdesc("one-arg", "printraw expects one argument exactly.")
	run("printraw", "even")
	setdesc("two-args", "printraw expects one argument exactly.")
	run("printraw", "even", "odd")
	setdesc("glob-arg", "printraw expects one argument exactly. Doesn't take globs.")
	run("printraw", "ev*")

	var seqkvs []keyvalue.KV

	group = "cmd-diff"
	setdesc("base-no-args", "Diffing base against base without args should have no diff.")
	run("diff")
	setdesc("changed-no-args", "Diffing base against changed without args should print all diffs.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("diff")
	setdesc("changed-no-context", "Diffing without context.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("-context=0", "diff")
	setdesc("keyptr", "Diff with -keyptr should diff only the target effect.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("-keyptr=html.ptr", "diff")
	setdesc("changed-with-template", "This is a rename example with a -template flag.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("-template=odd", "diff", "prime*")
	setdesc("changed-with-color", "Diffing base against changed but with colorization enabled.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("-color=yes", "diff")
	setdesc("changed-rmall", "Diffing base against changed with a .* removal regex.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("-x=.*", "diff")
	setdesc("changed-glob-arg", "Diffing base against changed with a glob should print all diffs for effects starting with 'even'.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("diff", "even*")
	setdesc("nonexistent-baseline", "Diffing against a baseline that doesn't exist.")
	fetchVersion = "nonexistent"
	run("diff")
	{
		setdesc("bad-baseline", "Diffing against a baseline that can't be parsed.")
		badkvs := append(slices.Clone(numsbase), keyvalue.KV{"aaa", "somevalue"})
		gz, err := edmain.Compress(badkvs, '=', edmain.Hash(badkvs))
		if err != nil {
			return nil, fmt.Errorf("effdumptest/compress badkvs: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpdir, "badkvs.gz"), gz, 0o644); err != nil {
			return nil, fmt.Errorf("effdumptest/write badkvs.gz: %v", err)
		}
		fetchVersion = "badkvs"
		run("diff")
	}
	setdesc("diff-by-default", "Diff is the default command to run in unclean clients.")
	fetchDirty = true
	p.Effects = slices.Clone(numsbase)
	p.Effects[1].V += "10\n"
	run()
	{
		setdesc("large", "Diffing large number of similar diffs.")
		n, content := 20, "1\n2\n3\n4\n5\n6\n7\n8\n"
		for i := 0; i < n; i++ {
			seqkvs = append(seqkvs, keyvalue.KV{strconv.Itoa(i + 10), content})
		}
		gz, err := edmain.Compress(seqkvs, '=', edmain.Hash(seqkvs))
		if err != nil {
			return nil, fmt.Errorf("effdumptest/compress seqkvs: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpdir, "seqkvs.gz"), gz, 0o644); err != nil {
			return nil, fmt.Errorf("effdumptest/write seqkvs.gz: %v", err)
		}
		for i := range seqkvs {
			seqkvs[i].V += "9\n"
		}
		fetchVersion, p.Effects = "seqkvs", seqkvs
		run("diff")
	}

	group = "cmd-diffkeys"
	setdesc("base-no-args", "Diffing base against base without args should have no diff.")
	run("diffkeys")
	setdesc("changed-no-args", "Diffing base against changed without args should print all diffs.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("diffkeys")
	setdesc("changed-glob-arg", "Diffing base against changed with a glob should print all diffs for effects starting with 'even'.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("diffkeys", "even*")

	group = "cmd-htmldiff"
	setdesc("base-no-args", "Diffing base against base without args should have no diff.")
	run("htmldiff")
	setdesc("changed-no-args", "Diffing base against changed without args should print all diffs.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("htmldiff")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	setdesc("changed-no-context", "Diffing without context.")
	run("-context=0", "htmldiff")
	setdesc("changed-rmall", "Diffing base against changed with a .* removal regex.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("-x=.*", "htmldiff")
	setdesc("changed-glob-arg", "Diffing base against changed with a glob should print all diffs for effects starting with 'ht'.")
	p.Effects = edtextar.Parse(nil, testdata("numschanged.textar"))
	run("htmldiff", "ht*")
	setdesc("large", "Diffing large number of similar diffs.")
	fetchVersion, p.Effects = "seqkvs", seqkvs
	run("htmldiff")

	group = "cmd-hash"
	setdesc("no-args", "Print the hash of the nums effdump.")
	run("hash")
	setdesc("some-args", "Subcommand hash doesn't take args")
	run("hash", "even")

	group = "cmd-save"
	setdesc("with-args", "Save cannot take args.")
	run("save", "somearg")
	setdesc("unclean-save-not-forced", "Save in unclean client needs -force.")
	fetchVersion, fetchDirty = "saved", true
	run("save")
	setdesc("unclean-save-forced", "Save in unclean client works with -forced.")
	fetchVersion, fetchDirty = "saved", true
	run("-force", "save")
	setdesc("clean-save-skipped", "Save of an already existing file with the same hash is skipped.")
	fetchVersion, fetchDirty = "saved", false
	run("save")
	setdesc("clean-save-rewritten", "Save of an already existing file with different hash is not skipped.")
	os.Truncate(filepath.Join(tmpdir, "saved.gz"), 0)
	fetchVersion, fetchDirty = "saved", false
	run("save")
	setdesc("save-by-default", "Save is the default command in clean commands.")
	fetchVersion, fetchDirty = "saved", false
	run()

	group = "cmd-clear"
	setdesc("with-args", "Clear cannot take args.")
	run("clear", "somearg")
	setdesc("normal", "Clear deletes the files.")
	run("clear")
	setdesc("empty", "Clear deletes nothing.")
	run("clear")

	return d, nil
}

func main() {
	d, err := mkdump()
	if err != nil {
		fmt.Fprintf(os.Stderr, "main run mkdump: %v", err)
		os.Exit(1)
	}
	d.Run(context.Background())
}
