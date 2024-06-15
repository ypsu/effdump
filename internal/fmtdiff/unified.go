// Package fmtdiff formats diffs.
package fmtdiff

import (
	"fmt"
	"strings"

	"github.com/ypsu/effdump/internal/andiff"
)

// Unified prints unified diff, suitable for terminal output.
func Unified(d andiff.Diff) string {
	w := &strings.Builder{}
	w.Grow(256)
	for i := 0; i < d.Ops[0]; i++ {
		fmt.Fprintf(w, " %s\n", d.LT[i])
	}
	x, y := d.LT[d.Ops[0]:], d.RT[d.Ops[0]:]
	for i := 1; i < len(d.Ops); i += 3 {
		for j := 0; j < d.Ops[i]; j++ {
			fmt.Fprintf(w, "-%s\n", x[j])
		}
		for j := 0; j < d.Ops[i+1]; j++ {
			fmt.Fprintf(w, "+%s\n", y[j])
		}
		for j := 0; j < d.Ops[i+2]; j++ {
			fmt.Fprintf(w, " %s\n", x[d.Ops[i]+j])
		}
		x, y = x[d.Ops[i]+d.Ops[i+2]:], y[d.Ops[i+1]+d.Ops[i+2]:]
	}
	return w.String()
}
