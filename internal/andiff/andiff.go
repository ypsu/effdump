// Package andiff implements the O(nlogn) anchored diff algorithm.
// Shamefully stolen from Go's src/internal/diff/diff.go.
// Heavily modified to better match effdump's needs.
package andiff

import (
	"hash/fnv"
	"regexp"
	"slices"
	"sort"
	"strings"
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

	Hash uint64
}

// A pair is a pair of values tracked for both the x and y side of a diff.
// It is typically a pair of line indexes.
type pair struct{ x, y int }

func split(s string) []string {
	ss := strings.Split(s, "\n")
	// Remove last "" but only if we have at least 2 lines so that we always return a non-empty slice.
	if len(ss) >= 2 && ss[len(ss)-1] == "" {
		return ss[:len(ss)-1]
	}
	return ss
}

// Compute computes the Diff between two strings.
// If rmregexp is non-nil, the matching parts are removed from each line first.
func Compute(lt, rt string, rmregexp *regexp.Regexp) Diff {
	origx, origy := split(lt), split(rt)
	x, y := origx, origy
	if rmregexp != nil {
		x, y = slices.Clone(x), slices.Clone(y)
		for i, s := range x {
			x[i] = rmregexp.ReplaceAllString(s, "")
		}
		for i, s := range y {
			y[i] = rmregexp.ReplaceAllString(s, "")
		}
	}
	if strings.Join(x, "\n") == strings.Join(y, "\n") {
		return Diff{origx, origy, []Op{{0, 0, len(x)}}, 0}
	}
	h := fnv.New64()

	var (
		ms     = tgs(x, y)        // matched lines
		ops    = make([]Op, 0, 3) // the result
		xi, yi = 0, 0             // the already processed lines
	)

	// Determine the common lines at the top.
	for xi < len(x) && yi < len(y) && x[xi] == y[yi] {
		xi, yi = xi+1, yi+1
	}
	if xi > 0 {
		ops = append(ops, Op{0, 0, xi})
	}

	for xi < len(x) && yi < len(y) {
		// x[xi] and y[yi] is now not equal.

		// Go to the next matching unique line.
		for ms[0].x < xi || ms[0].y < yi {
			ms = ms[1:]
		}

		// m[0] is now the next matching unique line.
		// Find the highest matching line above it, not necessary unique.
		// And then find the next differing line too.
		nxi, nyi, dxi, dyi := ms[0].x, ms[0].y, ms[0].x, ms[0].y
		for nxi > xi && nyi > yi && x[nxi-1] == y[nyi-1] {
			nxi, nyi = nxi-1, nyi-1
		}
		for dxi < len(x) && dyi < len(y) && x[dxi] == y[dyi] {
			dxi, dyi = dxi+1, dyi+1
		}

		// Try sliding the pure additions and pure removals up to find the best slide.
		// Best slide is the bottomest line with the min indentation while ignoring empty and whitspace only lines.
		// This is for handling the " [\n   a\n ]\n+[\n+  b\n+]\n [\n   c\n ]\n" case.
		var bestindent, bestslide, maxslide int
		if len(ops) > 0 {
			maxslide = ops[len(ops)-1].Keep
		}
		if nyi-yi == 0 { // pure deletion
			bestindent = countIndent(x[xi])
			for slide := 1; slide < maxslide && x[nxi-slide] == x[xi-slide]; slide++ {
				if indent := countIndent(x[xi-slide]); indent < bestindent {
					bestindent, bestslide = indent, slide
				}
			}
		}
		if nxi-xi == 0 { // pure addition
			bestindent = countIndent(y[yi])
			for slide := 1; slide < maxslide && y[nyi-slide] == y[yi-slide]; slide++ {
				if indent := countIndent(y[yi-slide]); indent < bestindent {
					bestindent, bestslide = indent, slide
				}
			}
		}
		if s := bestslide; s > 0 {
			ops[len(ops)-1].Keep -= s
			xi, yi, nxi, nyi, dxi, dyi = xi-s, yi-s, nxi-s, nyi-s, dxi-s, dyi-s
		}

		// Try very dumb heuristic for splitting the diff further if possible.
		// This improves a few more edge cases without adding much complexity.
		for txi, tyi := xi, yi; txi < nxi && tyi < nyi; txi, tyi = txi+1, tyi+1 {
			same := 0
			for x[txi+same] == y[tyi+same] {
				same++
			}
			if same > 0 {
				ops, xi, yi, txi, tyi = append(ops, Op{txi - xi, tyi - yi, same}), txi+same, tyi+same, txi+same, tyi+same
			}
		}

		for i := xi; i < nxi; i++ {
			h.Write([]byte("\n-"))
			h.Write([]byte(x[i]))
		}
		for i := yi; i < nyi; i++ {
			h.Write([]byte("\n+"))
			h.Write([]byte(y[i]))
		}
		ops = append(ops, Op{nxi - xi, nyi - yi, dxi - nxi})
		xi, yi = dxi, dyi
	}

	// Add the final operation block if needed.
	if xi < len(x) || yi < len(y) {
		for i := xi; i < len(x); i++ {
			h.Write([]byte("\n-"))
			h.Write([]byte(x[i]))
		}
		for i := yi; i < len(y); i++ {
			h.Write([]byte("\n+"))
			h.Write([]byte(y[i]))
		}
		ops = append(ops, Op{len(x) - xi, len(y) - yi, 0})
	}
	return Diff{origx, origy, ops, h.Sum64()}
}

func countIndent(s string) int {
	if strings.TrimSpace(s) == "" {
		return 1 << 30
	}
	indent := 0
	for strings.HasPrefix(s, " ") || strings.HasPrefix(s, "\t") {
		s, indent = s[1:], indent+1
	}
	return indent
}

// tgs returns the pairs of indexes of the longest common subsequence
// of unique lines in x and y, where a unique line is one that appears
// once in x and once in y.
//
// The longest common subsequence algorithm is as described in
// Thomas G. Szymanski, "A Special Case of the Maximal Common
// Subsequence Problem," Princeton TR #170 (January 1975),
// available at https://research.swtch.com/tgs170.pdf.
//
// Stolen (almost) as is from Go's internal code.
// The xi is [0,1,2,...,n] and then the matching yi is just a permutation of that sequence.
// Then it's just a standard nlogn longest-increasing-sequence algorithm.
// It's a pretty neatly implemented, only uses one map and reuses map indices to reduce allocations.
func tgs(x, y []string) []pair {
	// Count the number of times each string appears in a and b.
	// We only care about 0, 1, many, counted as 0, -1, -2
	// for the x side and 0, -4, -8 for the y side.
	// Using negative numbers now lets us distinguish positive line numbers later.
	m := make(map[string]int)
	for _, s := range x {
		if c := m[s]; c > -2 {
			m[s] = c - 1
		}
	}
	for _, s := range y {
		if c := m[s]; c > -8 {
			m[s] = c - 4
		}
	}

	// Now unique strings can be identified by m[s] = -1+-4.
	//
	// Gather the indexes of those strings in x and y, building:
	//	xi[i] = increasing indexes of unique strings in x.
	//	yi[i] = increasing indexes of unique strings in y.
	//	inv[i] = index j such that x[xi[i]] = y[yi[j]].
	var xi, yi, inv []int
	for i, s := range y {
		if m[s] == -1+-4 {
			m[s] = len(yi)
			yi = append(yi, i)
		}
	}
	for i, s := range x {
		if j, ok := m[s]; ok && j >= 0 {
			xi = append(xi, i)
			inv = append(inv, j)
		}
	}

	// Apply Algorithm A from Szymanski's paper.
	// In those terms, A = J = inv and B = [0, n).
	// We add sentinel pairs {0,0}, and {len(x),len(y)}
	// to the returned sequence, to help the processing loop.
	J := inv
	n := len(xi)
	T := make([]int, n)
	L := make([]int, n)
	for i := range T {
		T[i] = n + 1
	}
	for i := 0; i < n; i++ {
		k := sort.Search(n, func(k int) bool {
			return T[k] >= J[i]
		})
		T[k] = J[i]
		L[i] = k + 1
	}
	k := 0
	for _, v := range L {
		if k < v {
			k = v
		}
	}
	seq := make([]pair, 1+k)
	seq[k] = pair{len(x), len(y)} // sentinel at end
	lastj := n
	for i := n - 1; i >= 0; i-- {
		if L[i] == k && J[i] < lastj {
			k, seq[k-1] = k-1, pair{xi[i], yi[J[i]]}
		}
	}
	return seq
}
