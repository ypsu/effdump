package fmtdiff

import (
	"bytes"
	"fmt"
	"html"
	"strconv"
	"strings"

	_ "embed"

	"github.com/ypsu/effdump/internal/andiff"
)

//go:embed header.html
var htmlHeader []byte

//go:embed header.js
var jsHeader []byte

func cond[T any](c bool, ontrue, onfalse T) T {
	if c {
		return ontrue
	}
	return onfalse
}

func zip(op andiff.Op, last bool, contextLines int) (pre, zipped, post int) {
	const minzip = 4 // minimum lines to zip, no zipping below this count
	if op.Keep < contextLines+minzip {
		return op.Keep, 0, 0
	}
	if op.Del == 0 && op.Add == 0 {
		return 0, op.Keep - contextLines, contextLines
	}
	if last {
		return contextLines, op.Keep - contextLines, 0
	}
	if op.Keep < 2*contextLines+minzip {
		return op.Keep, 0, 0
	}
	return contextLines, op.Keep - 2*contextLines, contextLines
}

// HTMLBuckets formats a list of diff buckets into a HTML document.
func HTMLBuckets(buckets []Bucket) string {
	contextLines := 3
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
	printf("<script>\n%s</script>\n\n", jsHeader)

	// Render the diff table.
	for bucketid, bucket := range buckets {
		summarized := len(bucket.Entries) >= 10
		printf("<p>bucket <a id=b%d href='#b%d'>#%d</a>: %d diffs</p>\n", bucketid+1, bucketid+1, bucketid+1, len(bucket.Entries))
		printf("<ul>\n")
		for entryidx, entry := range bucket.Entries {
			if summarized && entryidx == 7 {
				printf("  <li><details><summary>... (additional %d similar diffs)</summary>\n", len(bucket.Entries)-entryidx)
			}
			printf("  <li><details%s><summary>%s</summary><table>\n", cond(entryidx == 0, " open", ""), html.EscapeString(entry.Name))

			x, xi, y, yi := entry.Diff.LT, 0, entry.Diff.RT, 0
			for opidx, op := range entry.Diff.Ops {
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

				pre, zipped, post := zip(op, opidx == len(entry.Diff.Ops)-1, contextLines)
				for i, k := 0, pre; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class=cSide>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class=cSide>%s</td>\n", html.EscapeString(y[yi]))
					xi, yi = xi+1, yi+1
				}
				if zipped > 0 {
					printf("    <tr>\n")
					printf("      <td class='cZipped cfgNeutral' colspan=2><button title=Expand onclick=expand(event)>&nbsp;↕&nbsp;</button> @@ %d common lines @@</td>\n", zipped)
					for i, k := 0, zipped; i < k; i++ {
						printf("    <tr hidden>\n")
						printf("      <td class=cSide>%s</td>\n", html.EscapeString(x[xi]))
						printf("      <td class=cSide>%s</td>\n", html.EscapeString(y[yi]))
						xi, yi = xi+1, yi+1
					}
				}
				for i, k := 0, post; i < k; i++ {
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
