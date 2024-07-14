package fmtdiff

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	_ "embed"

	"github.com/ypsu/effdump/internal/andiff"
)

//go:embed header.html
var htmlHeader string

//go:embed header.js
var jsHeader string

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
func HTMLBuckets(buckets []Bucket, contextLines int) string {
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

	// Render the header.
	replacer := strings.NewReplacer(
		"${SIDEWIDTH}", strconv.Itoa(min(120, width)+2),
		"${FULLWIDTH}", strconv.Itoa(width+2),
	)
	printf("%s\n", replacer.Replace(htmlHeader))
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
					printf("      <td class='cNum cbgNegative'>%d</td>\n", xi+1)
					printf("      <td class='cLeft cbgNegative'>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class='cNum cbgPositive'>%d</td>\n", yi+1)
					printf("      <td class='cRight cbgPositive'>%s</td>\n", html.EscapeString(y[yi]))
					xi, yi = xi+1, yi+1
				}

				for i, k := op.Add, op.Del; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class='cNum cbgNegative'>%d</td>\n", xi+1)
					printf("      <td class='cLeft cbgNegative'>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class='cNum cbgNeutral'> </td>\n")
					printf("      <td class='cRight cbgNeutral'></td>\n")
					xi++
				}

				for i, k := op.Del, op.Add; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class='cNum cbgNeutral'> </td>\n")
					printf("      <td class='cLeft cbgNeutral'></td>\n")
					printf("      <td class='cNum cbgPositive'>%d</td>\n", yi+1)
					printf("      <td class='cRight cbgPositive'>%s</td>\n", html.EscapeString(y[yi]))
					yi++
				}

				pre, zipped, post := zip(op, opidx == len(entry.Diff.Ops)-1, contextLines)
				for i, k := 0, pre; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class=cNum>%d</td>\n", xi+1)
					printf("      <td class=cLeft>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class=cNum>%d</td>\n", yi+1)
					printf("      <td class=cRight>%s</td>\n", html.EscapeString(y[yi]))
					xi, yi = xi+1, yi+1
				}
				if zipped > 0 {
					printf("    <tr>\n")
					hdr := hunkheader{}
					for k := 0; k < zipped; k++ {
						hdr.improve(y[yi+k])
					}
					hdrs := html.EscapeString(hdr.header(y[yi+zipped:]))
					printf("      <td class='cZipped cfgNeutral' colspan=4><button title=Expand onclick=expand(event)>&nbsp;↕&nbsp;</button> @@ %d common lines @@%s</td>\n", zipped, hdrs)
					for i, k := 0, zipped; i < k; i++ {
						printf("    <tr hidden>\n")
						printf("      <td class=cNum>%d</td>\n", xi+1)
						printf("      <td class=cLeft>%s</td>\n", html.EscapeString(x[xi]))
						printf("      <td class=cNum>%d</td>\n", yi+1)
						printf("      <td class=cRight>%s</td>\n", html.EscapeString(y[yi]))
						xi, yi = xi+1, yi+1
					}
				}
				for i, k := 0, post; i < k; i++ {
					printf("    <tr>\n")
					printf("      <td class=cNum>%d</td>\n", xi+1)
					printf("      <td class=cLeft>%s</td>\n", html.EscapeString(x[xi]))
					printf("      <td class=cNum>%d</td>\n", yi+1)
					printf("      <td class=cRight>%s</td>\n", html.EscapeString(y[yi]))
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
