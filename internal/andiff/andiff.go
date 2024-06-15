// Package andiff implements the anchored diff algorithm.
// Shamefully stolen from Go's src/internal/diff/diff.go.
// Heavily modified to better match effdump's needs.
package andiff

import (
	"strings"

	"github.com/ypsu/effdump/internal/edbg"
)

// Diff describes a diff.
type Diff struct {
	// The individual left and right lines.
	LT, RT []string

	// The zig-zag representation of the diff operations in terms of lines.
	// It's [common, left-remove, right-add, common, left-remove, ..., right-add, common].
	// E.g. [2, 1, 1, 3] means 2 common lines at top, one changed line, and 3 common lines at the bottom.
	// Or [5] means there's no diff and both sides have the same 5 lines.
	// The length of this slice is 3*k+1 where k is the number of diff chunks.
	Ops []int
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
		return Diff{x, y, []int{len(x)}}
	}
	var topcomm, botcomm int
	for topcomm < min(len(x), len(y)) && x[topcomm] == y[topcomm] {
		topcomm++
	}
	for botcomm < min(len(x), len(y)) && len(x)-botcomm > topcomm && len(y)-botcomm > topcomm && x[len(x)-botcomm-1] == y[len(y)-botcomm-1] {
		botcomm++
	}
	d := Diff{x, y, []int{topcomm, len(x) - topcomm - botcomm, len(y) - topcomm - botcomm, botcomm}}
	if edbg.Printf != nil {
		// Dump diff data for debugging.
		edbg.Printf("Diff data, len(LT):%d, len(RT):%d, topcommon:%d,\n", len(d.LT), len(d.RT), d.Ops[0])
		for i := 1; i < len(d.Ops); i += 3 {
			edbg.Printf("  lt:%d rt:%d common:%d\n", d.Ops[i], d.Ops[i+1], d.Ops[i+2])
		}
	}
	return d
}
