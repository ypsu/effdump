// Package marshal converts between key/value pairs and a binary representation.
//
// The effdump0 file format consists of 3 parts:
//
//   - The "effdump0" identifier at the beginning.
//   - 4 byte little endian length of the uncompressed stream.
//     No support for more than 1 GiB dumps, they would be hard to diff anyway.
//     Large dumps are expected to split or sharded into smaller dumps.
//   - Flate compressed stream of the key value pairs.
//
// The uncompressed stream is just entries appended after each other.
// The entries are sorted by their key part.
// Each entry consists of 4 parts:
//
// - 2 byte little endian length of the key.
// - The key value.
// - 4 byte little endian length of the value.
// - The value part.
package marshal

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
)

// Entry represent a single key-value entry in/from the dump file.
type Entry struct {
	Key, Value string
}

// Entries is a collection of Entry objects.
// This list must be sorted according to the key.
type Entries []Entry

// MarshalBinary encodes `es` into a byte stream suitable for saving to disk.
// Returns an error if `es` hits some internal limits.
func (es *Entries) MarshalBinary() (data []byte, err error) {
	dumplen := 6 * len(*es)
	for _, e := range *es {
		dumplen += len(e.Key) + len(e.Value)
		if len(e.Key) >= 1<<15 {
			return nil, fmt.Errorf("marshal.MarshalBinary: key %q length is %d, limit is %d", e.Key[:15]+"...", len(e.Key), 1<<15)
		}
	}
	if dumplen > 1<<30 {
		return nil, fmt.Errorf("marshal.MarshalBinary: effdump size is %d, limit is 1 GiB", dumplen)
	}
	w := bytes.NewBuffer(make([]byte, 0, 12+dumplen))
	fmt.Fprint(w, "effdump0")
	binary.Write(w, binary.LittleEndian, int32(dumplen))

	cw, _ := flate.NewWriter(w, flate.DefaultCompression)
	for _, e := range *es {
		binary.Write(cw, binary.LittleEndian, int16(len(e.Key)))
		cw.Write([]byte(e.Key))
		binary.Write(cw, binary.LittleEndian, int32(len(e.Value)))
		cw.Write([]byte(e.Value))
	}
	cw.Flush()
	return w.Bytes(), nil
}
