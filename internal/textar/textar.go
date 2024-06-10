// Package textar (TEXT ARchive) encodes/decodes key value string pairs into/from one large string.
package textar

import (
	"fmt"
	"strings"

	"github.com/ypsu/effdump/internal/effect"
)

func quote(s string) string {
	q := fmt.Sprintf("%q", s)
	return q[1 : len(q)-1]
}

func unquote(q string) string {
	var s string
	fmt.Sscanf(`"`+q+`"`, "%q", &s)
	return s
}

// Format encodes key value pairs into a textar string.
func Format(es []effect.Effect) string {
	// Pick a unique separator string that no value contains.
	maxdash, bufsz := 0, 0
	for _, e := range es {
		bufsz += len(e.Key) + len(e.Value) + 2
		d := 0
		for i := 0; i < len(e.Value); i++ {
			switch e.Value[i] {
			case '\n':
				d = 0
			case '-':
				d, maxdash = d+1, max(maxdash, d+1)
			default:
				d = -99999999
			}
		}
	}
	sep := strings.Repeat("-", max(3, maxdash+2)) + " "
	bufsz += len(es) * len(sep)

	w := strings.Builder{}
	w.Grow(bufsz)
	for i, e := range es {
		if i > 0 {
			w.WriteString("\n")
		}
		w.WriteString(sep)
		w.WriteString(quote(e.Key))
		w.WriteString("\n")
		w.WriteString(e.Value)
	}
	return w.String()
}

// Parse decodes a textar string into key value pairs.
func Parse(ar string) []effect.Effect {
	var es []effect.Effect
	sep, rest, ok := strings.Cut(ar, " ")
	sep = "\n" + sep + " "
	for ok {
		var key, value string
		key, rest, ok = strings.Cut(rest, "\n")
		if !ok {
			return es
		}
		value, rest, ok = strings.Cut(rest, sep)
		es = append(es, effect.Effect{unquote(key), value})
	}
	return es
}
