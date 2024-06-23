// Package andiff implements the anchored diff algorithm.
// Shamefully stolen from Go's src/internal/diff/diff.go.
// Heavily modified to better match effdump's needs.
package andiff

import (
	"strings"

	"github.com/ypsu/effdump/internal/edbg"
)

// Op describes a single diff operation / transformation.
type Op struct {
	Del, Add, Keep int
}

// Diff describes a diff.
type Diff struct {
	// The individual left and right lines.
	LT, RT []string

	Ops []Op
}

func split(s string) []string {
	ss := strings.Split(s, "\n")
	// Remove last "" but only if we have at least 2 lines so that we always return a non-empty slice.
	if len(ss) >= 2 && ss[len(ss)-1] == "" {
		return ss[:len(ss)-1]
	}
	return ss
}

// Compute computes the Diff between two strings.
func Compute(lt, rt string) Diff {
	x, y := split(lt), split(rt)
	if lt == rt {
		return Diff{x, y, []Op{{0, 0, len(x)}}}
	}
	var topcomm, botcomm int
	for topcomm < min(len(x), len(y)) && x[topcomm] == y[topcomm] {
		topcomm++
	}
	for botcomm < min(len(x), len(y)) && len(x)-botcomm > topcomm && len(y)-botcomm > topcomm && x[len(x)-botcomm-1] == y[len(y)-botcomm-1] {
		botcomm++
	}
	ops := make([]Op, 0, 2)
	if topcomm > 0 {
		ops = append(ops, Op{0, 0, topcomm})
	}
	ops = append(ops, Op{len(x) - topcomm - botcomm, len(y) - topcomm - botcomm, botcomm})
	d := Diff{x, y, ops}
	if edbg.Printf != nil {
		// Dump diff data for debugging.
		if false {
			// TODO: Switch to this.
			edbg.Printf("Diff data, len(LT):%d, len(RT):%d\n", len(d.LT), len(d.RT))
			for _, op := range d.Ops {
				edbg.Printf("  del:%d add:%d keep:%d\n", op.Del, op.Add, op.Keep)
			}
		}
		edbg.Printf("Diff data, len(LT):%d, len(RT):%d, topcommon:%d,\n", len(d.LT), len(d.RT), topcomm)
		for _, op := range d.Ops[1:] {
			edbg.Printf("  lt:%d rt:%d common:%d\n", op.Del, op.Add, op.Keep)
		}
	}
	return d
}
