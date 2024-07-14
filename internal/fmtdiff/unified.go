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
func UnifiedBuckets(buckets []Bucket, sepch byte, contextLines int, colorize bool) string {
	var kvs []keyvalue.KV
	for bucketid, bucket := range buckets {
		e := bucket.Entries[0]
		title, diff := fmt.Sprintf("%s (%s, bucket %d)", e.Name, e.Comment, bucketid+1), Unified(e.Diff, contextLines, colorize)
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
func Unified(d andiff.Diff, contextLines int, colorize bool) string {
	var delColor, addColor, noticeColor, normalColor string
	if colorize {
		delColor, addColor = "\033[31m", "\033[32m"
		noticeColor, normalColor = "\033[33m", "\033[0m"
	}
	w := &strings.Builder{}
	w.Grow(256)
	x, y, xi, yi := d.LT, d.RT, 0, 0
	for i, op := range d.Ops {
		for xe := xi + op.Del; xi < xe; xi++ {
			fmt.Fprintf(w, "%s-%s%s\n", delColor, x[xi], normalColor)
		}
		for ye := yi + op.Add; yi < ye; yi++ {
			fmt.Fprintf(w, "%s+%s%s\n", addColor, y[yi], normalColor)
		}
		w.WriteString(normalColor)
		pre, zipped, post := zip(op, i == len(d.Ops)-1, contextLines)
		for ye := yi + pre; yi < ye; xi, yi = xi+1, yi+1 {
			fmt.Fprintf(w, " %s\n", y[yi])
		}
		if zipped > 0 {
			hdr := hunkheader{}
			for k := 0; k < zipped; k++ {
				hdr.improve(y[yi+k])
			}
			fmt.Fprintf(w, "%s@@ %d common lines @@%s%s\n", noticeColor, zipped, hdr.header(y[yi+zipped:]), normalColor)
			xi, yi = xi+zipped, yi+zipped
		}
		for ye := yi + post; yi < ye; xi, yi = xi+1, yi+1 {
			fmt.Fprintf(w, " %s%s\n", y[yi], normalColor)
		}
	}
	return w.String()
}
