package fmtdiff

import (
	"fmt"
	"html"
	"io"
	"slices"

	"github.com/ypsu/effdump/internal/andiff"
)

// HTMLFormatter collects html-formatted diffs.
type HTMLFormatter struct {
	lines map[string]int
	diffs map[string]andiff.Diff
}

// NewHTMLFormatter creates a new HTMLFormatter.
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		lines: map[string]int{},
		diffs: map[string]andiff.Diff{},
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
	hf.diffs[name] = d
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

	lines := make([]string, 0, len(hf.lines))
	for line := range hf.lines {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	printf("let lines = [\n")
	for i, line := range lines {
		hf.lines[line] = i
		printf("'%s',\n", html.EscapeString(line))
	}
	printf("]\n")

	return int64(sw.n), sw.err
}
