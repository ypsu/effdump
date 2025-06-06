// Package edtextar (EffDump TEXT ARchive) encodes/decodes key value string pairs into/from one large string.
// This is a simplified version of https://github.com/ypsu/textar to avoid the unnecessary dependency.
package edtextar

import (
	"strings"

	"github.com/ypsu/effdump/internal/keyvalue"
)

// Format encodes key value pairs into a textar string.
// sepch is the separator character.
// Recommended to pass = or -.
func Format(kvs []keyvalue.KV, sepch byte) string {
	// Pick a unique separator string that no value contains.
	maxdash, bufsz := 0, 0
	for _, kv := range kvs {
		bufsz += len(kv.K) + len(kv.V) + 2
		d := 0
		for i := 0; i < len(kv.V); i++ {
			switch kv.V[i] {
			case '\n':
				d = 0
			case sepch:
				d, maxdash = d+1, max(maxdash, d+1)
			default:
				d = -99999999
			}
		}
	}
	sep := strings.Repeat(string(sepch), max(3, maxdash+2)) + " "
	bufsz += len(kvs) * len(sep)

	w := strings.Builder{}
	w.Grow(bufsz)
	for i, kv := range kvs {
		if i > 0 {
			w.WriteString("\n")
		}
		w.WriteString(sep)
		w.WriteString(strings.ReplaceAll(kv.K, "\n", "\\n"))
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
		dst = append(dst, keyvalue.KV{key, value})
	}
	return dst
}
