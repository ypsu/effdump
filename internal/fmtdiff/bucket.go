package fmtdiff

import "github.com/ypsu/effdump/internal/andiff"

// Entry is a andiff.Diff with a name associated.
type Entry struct {
	Name    string
	Comment string
	Diff    andiff.Diff
}

// Bucket contains Diffs that hash to the same value.
type Bucket struct {
	Hash    uint64
	Entries []Entry
}
