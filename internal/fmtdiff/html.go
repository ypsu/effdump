package fmtdiff

import (
	"bytes"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"

	_ "embed"
)

//go:embed header.html
var htmlHeader []byte

//go:embed render.js
var renderJS []byte

// HTMLBuckets formats a list of diff buckets into a HTML document.
func HTMLBuckets(buckets []Bucket) string {
	w := &strings.Builder{}
	w.Grow(1 << 20)
	printf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) }

	linesidx := make(map[string]int, 1024)
	for _, bucket := range buckets {
		for _, e := range bucket.Entries {
			for _, line := range e.Diff.LT {
				linesidx[line] = 0
			}
			for _, line := range e.Diff.RT {
				linesidx[line] = 0
			}
		}
	}
	width, lines := 40, make([]string, 0, len(linesidx))
	for line := range linesidx {
		width, lines = max(width, len(line)), append(lines, line)
	}
	width = min(120, width)

	printf("%s\n", bytes.ReplaceAll(htmlHeader, []byte("${WIDTH}"), []byte(strconv.Itoa(width+2))))
	printf("%s\n", renderJS)
	slices.Sort(lines)
	printf("let lines = [\n")
	for i, line := range lines {
		linesidx[line] = i
		printf("'%s\\n',\n", strings.ReplaceAll(html.EscapeString(line), "\\", "&#92;"))
	}
	printf("]\n")

	printf("let diffbuckets = [\n")
	for _, bucket := range buckets {
		printf("  [\n")
		for _, entry := range bucket.Entries {
			printf("    {\n    name: '%s',\n    lt: [", html.EscapeString(entry.Name))
			for _, line := range entry.Diff.LT {
				printf("%d,", linesidx[line])
			}
			printf("],\n      rt: [")
			for _, line := range entry.Diff.RT {
				printf("%d,", linesidx[line])
			}
			printf("],\n      ops: [")
			for _, op := range entry.Diff.Ops {
				printf("%d,%d,%d,", op.Del, op.Add, op.Keep)
			}
			printf("],\n    },\n")
		}
		printf("  ],\n")
	}
	printf("]\n")

	printf("\nmain()\n</script>\n</body>\n</html>")
	return w.String()
}
