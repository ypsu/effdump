// Package fmtdiff formats diffs.
package fmtdiff

import (
	"fmt"
	"io"
	"strings"

	"github.com/ypsu/effdump/internal/andiff"
	"github.com/ypsu/effdump/internal/keyvalue"
	"github.com/ypsu/effdump/internal/textar"
)

// UnifiedFormatter collects unified-formatted diffs into a textar.
type UnifiedFormatter struct {
	kvs     []keyvalue.KV
	sepchar byte
}

// NewUnifiedFormatter creates a new UnifiedFormatter.
func NewUnifiedFormatter(sepchar byte) *UnifiedFormatter {
	return &UnifiedFormatter{sepchar: sepchar}
}

// Add Adds a diff's unified representation to the result.
func (uf *UnifiedFormatter) Add(name string, d andiff.Diff) {
	difftext := Unified(d)
	if difftext != "" {
		difftext = "\t" + strings.ReplaceAll(difftext, "\n", "\n\t")
	}
	uf.kvs = append(uf.kvs, keyvalue.KV{name, difftext})
}

// WriteTo writes the resulting textar to w.
func (uf *UnifiedFormatter) WriteTo(w io.Writer) (int, error) {
	n, err := io.WriteString(w, textar.Format(uf.kvs, uf.sepchar))
	w.Write([]byte("\n"))
	return n + 1, err
}

// Unified prints unified diff, suitable for terminal output.
func Unified(d andiff.Diff) string {
	ctx := 3
	w := &strings.Builder{}
	w.Grow(256)
	ops, x, y, xi, yi, keep := d.Ops, d.LT, d.RT, 0, 0, 0
	if d.Ops[0].Del == 0 && d.Ops[0].Add == 0 && d.Ops[0].Keep > ctx+3 {
		fmt.Fprintf(w, "@@ %d common lines @@\n", d.Ops[0].Keep-ctx)
		xi, yi, keep, ops = d.Ops[0].Keep-ctx, d.Ops[0].Keep-ctx, ctx, ops[1:]
	}
	for _, op := range ops {
		if keep > 2*ctx+3 {
			for i := 0; i < ctx; i++ {
				fmt.Fprintf(w, " %s\n", x[xi+i])
			}
			fmt.Fprintf(w, "@@ %d common lines @@\n", keep-2*ctx)
			keep, xi, yi = ctx, xi+keep-ctx, yi+keep-ctx
		}
		for xe := xi + keep; xi < xe; xi++ {
			fmt.Fprintf(w, " %s\n", x[xi])
		}
		keep, yi = op.Keep, yi+keep
		for xe := xi + op.Del; xi < xe; xi++ {
			fmt.Fprintf(w, "-%s\n", x[xi])
		}
		for ye := yi + op.Add; yi < ye; yi++ {
			fmt.Fprintf(w, "+%s\n", y[yi])
		}
	}
	common := 0
	if keep > ctx+3 {
		keep, common = ctx, keep-ctx
	}
	for xe := xi + keep; xi < xe; xi++ {
		fmt.Fprintf(w, " %s\n", x[xi])
	}
	if common > 0 {
		fmt.Fprintf(w, "@@ %d common lines @@\n", common)
	}
	return w.String()
}
