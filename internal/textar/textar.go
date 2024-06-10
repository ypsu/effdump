// Package textar (TEXT ARchive) encodes/decodes key value string pairs into/from one large string.
package textar

import (
	"fmt"
	"strings"

	"github.com/ypsu/effdump/internal/keyvalue"
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
func Format(kvs []keyvalue.KV) string {
	// Pick a unique separator string that no value contains.
	maxdash, bufsz := 0, 0
	for _, kv := range kvs {
		bufsz += len(kv.K) + len(kv.V) + 2
		d := 0
		for i := 0; i < len(kv.V); i++ {
			switch kv.V[i] {
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
	bufsz += len(kvs) * len(sep)

	w := strings.Builder{}
	w.Grow(bufsz)
	for i, kv := range kvs {
		if i > 0 {
			w.WriteString("\n")
		}
		w.WriteString(sep)
		w.WriteString(quote(kv.K))
		w.WriteString("\n")
		w.WriteString(kv.V)
	}
	return w.String()
}

// Parse decodes a textar string into key value pairs.
// The decoded entries are appended to dst and then dst is returned.
// dst can be nil.
func Parse(dst []keyvalue.KV, ar string) []keyvalue.KV {
	sep, rest, ok := strings.Cut(ar, " ")
	sep = "\n" + sep + " "
	for ok {
		var key, value string
		key, rest, ok = strings.Cut(rest, "\n")
		if !ok {
			return dst
		}
		value, rest, ok = strings.Cut(rest, sep)
		dst = append(dst, keyvalue.KV{unquote(key), value})
	}
	return dst
}
