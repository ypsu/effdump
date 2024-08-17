// markdowndump is a demo.
// See README.md for the details.
package main

import (
	"context"
	"fmt"
	"strings"

	_ "embed"

	"github.com/ypsu/effdump"
)

//go:embed markdowndump.textar
var testdata string

// Markdown converts the input markdown to HTML.
func Markdown(md string) string {
	sb := &strings.Builder{}
	sb.WriteString("<html>\n")
	for _, para := range strings.Split(md, "\n\n") {
		para = strings.TrimRight(para, " \t\n")
		if para == "" {
			continue
		}
		switch para[0] {
		case '-':
			fmt.Fprintf(sb, "<ul>%s</ul>\n", strings.ReplaceAll("\n"+para, "\n- ", "\n<li>"))
		case ' ', '\t':
			fmt.Fprintf(sb, "<pre>%s</pre>\n", para)
		default:
			fmt.Fprintf(sb, "<p>%s</p>\n", para)
		}
	}
	return sb.String()
}

func makedump() *effdump.Dump {
	dump := effdump.New("markdowndump")
	testentries := strings.Split(testdata[3:], "\n== ")
	for _, entry := range testentries {
		key, value, _ := strings.Cut(entry, "\n")
		dump.Add(key, fmt.Sprintf("== input:\n%s\n\n== output:\n%s\n", value, Markdown(value)))
	}
	return dump
}

func main() {
	makedump().Run(context.Background())
}
