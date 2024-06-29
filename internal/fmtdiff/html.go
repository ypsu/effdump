package fmtdiff

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"slices"
	"strconv"
	"strings"

	_ "embed"

	"github.com/ypsu/effdump/internal/andiff"
)

//go:embed header.html
var htmlHeader []byte

//go:embed render.js
var renderJS []byte

type diffentry struct {
	name string
	diff andiff.Diff
}

// HTMLFormatter collects html-formatted diffs.
type HTMLFormatter struct {
	lines map[string]int
	diffs []diffentry
}

// NewHTMLFormatter creates a new HTMLFormatter.
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		lines: map[string]int{},
	}
}

// Add adds a diff's html representation to the result.
func (hf *HTMLFormatter) Add(name string, d andiff.Diff) {
	for _, line := range d.LT {
		hf.lines[line] = 0
	}
	for _, line := range d.RT {
		hf.lines[line] = 0
	}
	hf.diffs = append(hf.diffs, diffentry{name, d})
}

type safeWriter struct {
	w   io.Writer // the target
	n   int       // total written so far
	err error     // any error if occured
}

func (sf *safeWriter) printf(format string, args ...any) {
	if sf.err != nil {
		return
	}
	n, err := fmt.Fprintf(sf.w, format, args...)
	sf.n, sf.err = sf.n+n, err
}

// WriteTo writes the resulting html to w.
func (hf *HTMLFormatter) WriteTo(w io.Writer) (totalwritten int64, err error) {
	sw := &safeWriter{w: w}
	printf := sw.printf

	width, lines := 40, make([]string, 0, len(hf.lines))
	for line := range hf.lines {
		width, lines = max(width, len(line)), append(lines, line)
	}
	width = min(120, width)

	printf("%s\n", bytes.ReplaceAll(htmlHeader, []byte("${WIDTH}"), []byte(strconv.Itoa(width+2))))
	printf("%s\n", renderJS)
	slices.Sort(lines)
	printf("let lines = [\n")
	for i, line := range lines {
		hf.lines[line] = i
		printf("'%s\\n',\n", strings.ReplaceAll(html.EscapeString(line), "\\", "&#92;"))
	}
	printf("]\n")

	printf("let diffs = {\n")
	for _, de := range hf.diffs {
		printf("  '%s': {\n    lt: [", de.name)
		for _, line := range de.diff.LT {
			printf("%d,", hf.lines[line])
		}
		printf("],\n    rt: [")
		for _, line := range de.diff.RT {
			printf("%d,", hf.lines[line])
		}
		printf("],\n    ops: [")
		for _, op := range de.diff.Ops {
			printf("%d,%d,%d,", op.Del, op.Add, op.Keep)
		}
		printf("],\n  },\n")
	}
	printf("}\n")

	printf("\nmain()\n</script>\n</body>\n</html>")
	return int64(sw.n), sw.err
}
