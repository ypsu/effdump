// Package fmtdiff formats diffs.
package fmtdiff

import (
	"fmt"
	"strings"

	"github.com/ypsu/effdump/internal/andiff"
	"github.com/ypsu/effdump/internal/keyvalue"
	"github.com/ypsu/effdump/internal/textar"
)

// UnifiedBuckets formats a list of diff buckets into a textar.
func UnifiedBuckets(buckets []Bucket, sepch byte) string {
	var kvs []keyvalue.KV
	for bucketid, bucket := range buckets {
		e := bucket.Entries[0]
		title, diff := fmt.Sprintf("%s (%s, bucket %d)", e.Name, e.Comment, bucketid+1), Unified(e.Diff)
		if diff != "" {
			diff = "\t" + strings.ReplaceAll(diff, "\n", "\n\t")
		}
		kvs = append(kvs, keyvalue.KV{title, diff})
		cnt := len(bucket.Entries)
		if cnt == 1 {
			continue
		}
		tolist, title := cnt, fmt.Sprintf("(omitted %d similar diffs in bucket %d)", cnt-1, bucketid+1)
		if tolist >= 10 {
			tolist = 7
		}
		keys := make([]string, 0, len(bucket.Entries[1:tolist])+1)
		for _, e := range bucket.Entries[1:tolist] {
			keys = append(keys, e.Name)
		}
		if cnt > tolist {
			keys = append(keys, fmt.Sprintf("... (%d more entries)", cnt-tolist))
		}
		kvs = append(kvs, keyvalue.KV{title, "\t" + strings.Join(keys, "\n\t") + "\n"})
	}
	return textar.Format(kvs, sepch) + "\n"
}

// Unified returns unified diff, suitable for terminal output.
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
