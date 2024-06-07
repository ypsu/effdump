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
// - 4 byte little endian length of the value.
// - The key value.
// - The value part.
package marshal

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
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
		binary.Write(cw, binary.LittleEndian, int32(len(e.Value)))
		cw.Write([]byte(e.Key))
		cw.Write([]byte(e.Value))
	}
	cw.Close()
	return w.Bytes(), nil
}

// UnmarshalBinary decodes a byte stream into `es`.
func (es *Entries) UnmarshalBinary(data []byte) error {
	if !bytes.HasPrefix(data, []byte("effdump0")) {
		return fmt.Errorf("marshal.UnmarshalBinary: invalid header, want effdump0")
	}

	// Pre-allocate buffer for the result.
	w, r, resultSize := &strings.Builder{}, bytes.NewBuffer(data[8:]), int32(0)
	binary.Read(r, binary.LittleEndian, &resultSize)
	if resultSize < 0 {
		return fmt.Errorf("marshal.UnmarshalBinary: result size = %d, want non-negative", resultSize)
	}
	w.Grow(int(resultSize))

	// Decompress into the result.
	cr := flate.NewReader(r)
	copied, err := io.Copy(w, cr)
	if err != nil {
		return fmt.Errorf("marshal.UnmarshalBinary: io.Copy(): %v, uncompressed %d bytes", err, copied)
	}
	if copied != int64(resultSize) {
		return fmt.Errorf("marshal.UnmarshalBinary: uncompressed size is %d, want %d", copied, resultSize)
	}

	// Split the result into entries.
	nes, s, o := make([]Entry, 0, 16), w.String(), 0
	for o+6 <= len(s) {
		keysz, valuesz := int(binary.LittleEndian.Uint16([]byte(s[o:o+2]))), int(binary.LittleEndian.Uint16([]byte(s[o+2:o+6])))
		if o+keysz+valuesz > len(s) {
			return fmt.Errorf("marshal.UnmarshalBinary: entry at byte %d too large", o)
		}
		nes = append(nes, Entry{s[o+6 : o+6+keysz], s[o+6+keysz : o+6+keysz+valuesz]})
		o += 6 + keysz + valuesz
	}
	if o != len(s) {
		return fmt.Errorf("marshal.UnmarshalBinary: incomplete last entry")
	}

	*es = nes
	return nil
}
