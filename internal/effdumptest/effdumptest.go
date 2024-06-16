// Binary effdumptest generates the effdump library's effects.
package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ypsu/effdump"
	"github.com/ypsu/effdump/internal/edbg"
	"github.com/ypsu/effdump/internal/edmain"
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
		i int
		v []string
	}{{1, []string{"a", "b"}}, {2, []string{"multiline\nstring"}}})
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

	// Set up common helpers.
	desc, w, log := "", &strings.Builder{}, &strings.Builder{}
	fetchVersion, fetchClean, fetchErr := "dummyversion", true, error(nil)
	p := &edmain.Params{
		Name:   "testdump",
		Env:    []string{"EFFDUMP_DIR=" + tmpdir},
		Stdout: w,
		Sepch:  "-",

		FetchVersion: func(context.Context) (string, bool, error) {
			edbg.Printf("FetchVersion() -> (%q, %t, %v)\n", fetchVersion, fetchClean, fetchErr)
			return fetchVersion, fetchClean, fetchErr
		},

		ResolveVersion: func(_ context.Context, ref string) (string, error) {
			edbg.Printf("ResolveVersion(%s) -> (%q, %v)\n", ref, fetchVersion, fetchErr)
			return fetchVersion, fetchErr
		},
	}
	run := func() {
		kvs, name := make([]keyvalue.KV, 1, 4), ""
		desc = strings.ReplaceAll(desc, "\n\t\t", "\n") // Deindent.
		name, desc, _ = strings.Cut(desc, "\n")
		kvs[0] = keyvalue.KV{"desc", desc}
		err := p.Run(ctx)
		if w.Len() > 0 {
			kvs = append(kvs, keyvalue.KV{"stdout", w.String()})
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
		d.Add(name, textar.Format(kvs, '~'))
	}

	// The baseline for the following test will be numsbase.
	fetchVersion = "numsbase"
	gz, err := edmain.Compress(textar.Parse(nil, testdata("numsbase.textar")), '=')
	if err != nil {
		return nil, fmt.Errorf("effdumptest/compress numsbase: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpdir, "numsbase.gz"), gz, 0o644); err != nil {
		return nil, fmt.Errorf("effdumptest/write numsbase.gz: %v", err)
	}

	desc = `nums-print/no-args
		Printing without args should print all the effects.

			testdump print
	`
	p.Args = []string{"print"}
	p.Effects = textar.Parse(nil, testdata("numsbase.textar"))
	run()

	desc = `nums-print/two-args
		Printing without args should print only the even and odd effects.

			testdump print even odd
	`
	p.Args = []string{"print", "even", "odd"}
	p.Effects = textar.Parse(nil, testdata("numsbase.textar"))
	run()

	desc = `nums-print/oddglob-arg
		Printing without args should print all effects starting with "o*".

			testdump print odd*
	`
	p.Args = []string{"print", "odd*"}
	p.Effects = textar.Parse(nil, testdata("numsbase.textar"))
	run()

	desc = `nums-print/glob-arg
		Printing without args should print all effects containing "o".

			testdump print *o*
	`
	p.Args = []string{"print", "*o*"}
	p.Effects = textar.Parse(nil, testdata("numsbase.textar"))
	run()

	desc = `nums-print/dup-error
		There's a duplicate entry added in this one.

			testdump print
	`
	p.Args = []string{"print"}
	p.Effects = textar.Parse(nil, testdata("numsbase.textar"))
	p.Effects = append(p.Effects, keyvalue.KV{"all", "another all entry"})
	run()

	desc = `nums-diff/base-no-args
		Diffing base against base without args should have no output.

			testdump diff
	`
	p.Args = []string{"diff"}
	p.Effects = textar.Parse(nil, testdata("numsbase.textar"))
	run()

	desc = `nums-diff/changed-no-args
		Diffing base against changed without args should have print all diffs.

			testdump diff
	`
	p.Args = []string{"diff"}
	p.Effects = textar.Parse(nil, testdata("numschanged.textar"))
	run()

	desc = `nums-diff/changed-glob-arg
		Diffing base against changed without args should have print all diffs for effects starting with "even".

			testdump diff even*
	`
	p.Args = []string{"diff", "even*"}
	p.Effects = textar.Parse(nil, testdata("numschanged.textar"))
	run()

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
