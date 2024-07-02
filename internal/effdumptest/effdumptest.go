// Binary effdumptest generates the effdump library's effects.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ypsu/effdump"
	"github.com/ypsu/effdump/internal/andiff"
	"github.com/ypsu/effdump/internal/edbg"
	"github.com/ypsu/effdump/internal/edmain"
	"github.com/ypsu/effdump/internal/fmtdiff"
	"github.com/ypsu/effdump/internal/keyvalue"
	"github.com/ypsu/effdump/internal/textar"
)

//go:embed *.textar
var testdataFS embed.FS

func addStringifyEffects(d *effdump.Dump) {
	d.Add("stringify/int", 42)
	d.Add("stringify/byte", 'a')
	d.Add("stringify/stringer", time.UnixMilli(0).UTC())
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
	diffar := textar.Parse(nil, testdata("trickydiffs.textar"))
	for _, kv := range diffar {
		lt, rt := &strings.Builder{}, &strings.Builder{}
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
		diff := fmtdiff.Unified(andiff.Compute(lt.String(), rt.String()))
		kvs := make([]keyvalue.KV, 0, 3)
		kvs = append(kvs, keyvalue.KV{"input", kv.V})
		kvs = append(kvs, keyvalue.KV{"result", diff})
		kvs = append(kvs, keyvalue.KV{"debuglog", debuglog.String()})
		debuglog.Reset()
		d.Add("diffs/"+kv.K, textar.Format(kvs, '-'))
	}

	// Set up common helpers for the CLI tests.
	group, key, desc, w, log := "", "", "", &strings.Builder{}, &strings.Builder{}
	fetchVersion, fetchClean, fetchErr := "dummyversion", true, error(nil)
	p := &edmain.Params{
		Name:   "testdump",
		Env:    []string{"EFFDUMP_DIR=" + tmpdir},
		Stdout: w,

		FetchVersion: func(context.Context) (string, bool, error) {
			edbg.Printf("FetchVersion() -> (%q, %t, %v)\n", fetchVersion, fetchClean, fetchErr)
			return fetchVersion, fetchClean, fetchErr
		},

		ResolveVersion: func(_ context.Context, ref string) (string, error) {
			edbg.Printf("ResolveVersion(%s) -> (%q, %v)\n", ref, fetchVersion, fetchErr)
			return fetchVersion, fetchErr
		},
	}
	reset := func() {
		fetchVersion, fetchClean = "numsbase", true
		p.Effects = textar.Parse(nil, testdata("numsbase.textar"))
	}
	run := func(args ...string) {
		kvs := make([]keyvalue.KV, 1, 4)
		kvs[0] = keyvalue.KV{"desc", fmt.Sprintf("%s\n\nargs: testdump %q", desc, args)}
		fs := flag.NewFlagSet("effdumptest", flag.ContinueOnError)
		p.RegisterFlags(fs)
		if err := fs.Parse(args); err != nil {
			kvs = append(kvs, keyvalue.KV{"flagparse-error", err.Error()})
		}
		p.Args = fs.Args()
		p.Sepch = "-"
		p.OutputFile = "-"
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
		d.Add(key, textar.Format(kvs, '~'))

		// Reset defaults for the next testcase.
		reset()
	}
	setdesc := func(name, description string) { key, desc = fmt.Sprintf("%s/%s", group, name), description }

	// The baseline for the following tests will be numsbase.
	reset()
	numsbase := textar.Parse(nil, testdata("numsbase.textar"))
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
	setdesc("two-args", "Printing without args should print only the even and odd effects.")
	run("print", "even", "odd")
	setdesc("oddglob-arg", "Printing without args should print all effects starting with 'o*'.")
	run("print", "odd*")
	setdesc("glob-arg", "Printing without args should print all effects containing 'o'.")
	run("print", "*o*")
	setdesc("dup-error", "There's a duplicate entry added in this one.")
	p.Effects = append(p.Effects, keyvalue.KV{"all", "another all entry"})
	run("print")

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

	group = "cmd-diff"
	setdesc("base-no-args", "Diffing base against base without args should have no diff.")
	run("diff")
	setdesc("changed-no-args", "Diffing base against changed without args should print all diffs.")
	p.Effects = textar.Parse(nil, testdata("numschanged.textar"))
	run("diff")
	setdesc("changed-glob-arg", "Diffing base against changed with a glob should print all diffs for effects starting with 'even'.")
	p.Effects = textar.Parse(nil, testdata("numschanged.textar"))
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
	fetchClean = false
	p.Effects = slices.Clone(numsbase)
	p.Effects[1].V += "10\n"
	run()
	{
		setdesc("large", "Diffing large number of similar diffs.")
		n, content := 20, "1\n2\n3\n4\n5\n6\n7\n8\n"
		seqkvs := make([]keyvalue.KV, 0, n)
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

	group = "cmd-htmldiff"
	setdesc("base-no-args", "Diffing base against base without args should have no diff.")
	run("htmldiff")
	setdesc("changed-no-args", "Diffing base against changed without args should print all diffs.")
	p.Effects = textar.Parse(nil, testdata("numschanged.textar"))
	run("htmldiff")
	setdesc("changed-glob-arg", "Diffing base against changed with a glob should print all diffs for effects starting with 'ht'.")
	p.Effects = textar.Parse(nil, testdata("numschanged.textar"))
	run("htmldiff", "ht*")

	group = "cmd-hash"
	setdesc("no-args", "Print the hash of the nums effdump.")
	run("hash")
	setdesc("some-args", "Subcommand hash doesn't take args")
	run("hash", "even")

	group = "cmd-save"
	setdesc("with-args", "Save cannot take args.")
	run("save", "somearg")
	setdesc("unclean-save-not-forced", "Save in unclean client needs -force.")
	fetchVersion, fetchClean = "saved", false
	run("save")
	setdesc("unclean-save-forced", "Save in unclean client works with -forced.")
	fetchVersion, fetchClean = "saved", false
	run("-force", "save")
	setdesc("clean-save-skipped", "Save of an already existing file with the same hash is skipped.")
	fetchVersion, fetchClean = "saved", true
	run("save")
	setdesc("clean-save-rewritten", "Save of an already existing file with different hash is not skipped.")
	os.Truncate(filepath.Join(tmpdir, "saved.gz"), 0)
	fetchVersion, fetchClean = "saved", true
	run("save")
	setdesc("save-by-default", "Save is the default command in clean commands.")
	fetchVersion, fetchClean = "saved", true
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
