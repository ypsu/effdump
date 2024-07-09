package fmtdiff

import (
	"bytes"
	"fmt"
	"html"
	"strconv"
	"strings"

	_ "embed"
)

//go:embed header.html
var htmlHeader []byte

func cond[T any](c bool, ontrue, onfalse T) T {
	if c {
		return ontrue
	}
	return onfalse
}

// HTMLBuckets formats a list of diff buckets into a HTML document.
func HTMLBuckets(buckets []Bucket) string {
	w := &strings.Builder{}
	w.Grow(1 << 20)
	printf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) }

	// Compute the width of the columns.
	width := 40
	for _, bucket := range buckets {
		for _, e := range bucket.Entries {
			for _, line := range e.Diff.LT {
				width = max(width, len(line))
			}
			for _, line := range e.Diff.RT {
				width = max(width, len(line))
			}
		}
	}
	width = min(120, width)

	// Render the header.
	printf("%s\n", bytes.ReplaceAll(htmlHeader, []byte("${WIDTH}"), []byte(strconv.Itoa(width+2))))

	// Render the diff table.
	for bucketid, bucket := range buckets {
		summarized := len(bucket.Entries) >= 10
		printf("<p>bucket <a id=b%d href='#b%d'>#%d</a>: %d diffs</p>\n", bucketid+1, bucketid+1, bucketid+1, len(bucket.Entries))
		printf("<ul>\n")
		for entryidx, entry := range bucket.Entries {
			if summarized && entryidx == 7 {
				printf("  <li><details><summary>... (additional %d similar diffs))</summary>\n", len(bucket.Entries)-entryidx)
			}
			printf("  <li><details%s><summary>%s</summary><table>\n", cond(entryidx == 0, " open", ""), html.EscapeString(entry.Name))

			x, xi, y, yi := entry.Diff.LT, 0, entry.Diff.RT, 0
			for _, op := range entry.Diff.Ops {
				for i, k := 0, min(op.Del, op.Add); i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class='cSide cbgNegative'>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class='cSide cbgPositive'>%s</td>\n", html.EscapeString(y[yi]))
					xi, yi = xi+1, yi+1
				}

				for i, k := op.Add, op.Del; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class='cSide cbgNegative'>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class='cSide cbgNeutral'></td>\n")
					xi++
				}

				for i, k := op.Del, op.Add; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class='cSide cbgNeutral'></td>\n")
					printf("      <td class='cSide cbgPositive'>%s</td>\n", html.EscapeString(y[yi]))
					yi++
				}

				for i, k := 0, op.Keep; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class=cSide>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class=cSide>%s</td>\n", html.EscapeString(y[yi]))
					xi, yi = xi+1, yi+1
				}
			}
			printf("  </table></details>\n")
		}

		if summarized {
			printf("  </details>\n")
		}
		printf("</ul>\n<hr>\n\n")
	}

	printf("\n</body>\n</html>\n")
	return w.String()
}
