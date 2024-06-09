// Package effect defines Effect and a couple helper functions.
package effect

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// Effect represents a single key-value entry in an effdump
type Effect struct {
	Key, Value string
}

// Stringify converts any type to a string representation suitable to use in effects.
func Stringify(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	if bs, ok := v.([]byte); ok {
		return string(bs)
	}
	return fmt.Sprint(v)
}

// Marshal encodes `es` into an effdump0 byte stream suitable for saving to disk.
// Returns an error if `es` hits some internal limits.
//
// The effdump0 file format consists of 3 parts:
//
//   - The "effdump0" identifier at the beginning.
//   - 4 byte little endian length of the uncompressed stream.
//     No support for more than 1 GiB dumps, they would be hard to diff anyway.
//     Large dumps are expected to split or sharded into smaller dumps.
//   - Flate compressed stream of the key value pairs.
//
// The uncompressed stream is just effects appended after each other.
// The effects are sorted by their key part.
// Each effect consists of 4 parts in the encoded stream:
//
// - 2 byte little endian length of the key.
// - 4 byte little endian length of the value.
// - The key value.
// - The value part.
func Marshal(es []Effect) (data []byte, err error) {
	dumplen, lastKey := 6*len(es), ""
	for i, e := range es {
		dumplen += len(e.Key) + len(e.Value)
		if len(e.Key) >= 1<<15 {
			return nil, fmt.Errorf("effect marshal: key %q length is %d, limit is %d", e.Key[:15]+"...", len(e.Key), 1<<15)
		}
		if e.Key <= lastKey {
			return nil, fmt.Errorf("effect marshal: %dth not in sorted order", i)
		}
		lastKey = e.Key
	}
	if dumplen > 1<<30 {
		return nil, fmt.Errorf("effect marshal: effdump size is %d, limit is 1 GiB", dumplen)
	}
	w := bytes.NewBuffer(make([]byte, 0, 12+dumplen))
	fmt.Fprint(w, "effdump0")
	binary.Write(w, binary.LittleEndian, int32(dumplen))

	cw, _ := flate.NewWriter(w, flate.DefaultCompression)
	for _, e := range es {
		binary.Write(cw, binary.LittleEndian, int16(len(e.Key)))
		binary.Write(cw, binary.LittleEndian, int32(len(e.Value)))
		cw.Write([]byte(e.Key))
		cw.Write([]byte(e.Value))
	}
	cw.Close()
	return w.Bytes(), nil
}

// Unmarshal decodes an effdump0 byte stream into `es`.
// See Marshal() for info about the format.
func Unmarshal(data []byte) ([]Effect, error) {
	if !bytes.HasPrefix(data, []byte("effdump0")) {
		return nil, fmt.Errorf("effect unmarshal: invalid header, want effdump0")
	}

	// Pre-allocate buffer for the result.
	w, r, resultSize := &strings.Builder{}, bytes.NewBuffer(data[8:]), int32(0)
	binary.Read(r, binary.LittleEndian, &resultSize)
	if resultSize < 0 {
		return nil, fmt.Errorf("effect unmarshal: result size = %d, want non-negative", resultSize)
	}
	w.Grow(int(resultSize))

	// Decompress into the result.
	cr := flate.NewReader(r)
	copied, err := io.Copy(w, cr)
	if err != nil {
		return nil, fmt.Errorf("effect uncompress: %v, uncompressed %d bytes", err, copied)
	}
	if copied != int64(resultSize) {
		return nil, fmt.Errorf("effect unmarshal: uncompressed size is %d, want %d", copied, resultSize)
	}

	// Split the result into effects.
	es, s, o, lastKey := make([]Effect, 0, 16), w.String(), 0, ""
	for o+6 <= len(s) {
		keysz, valuesz := int(binary.LittleEndian.Uint16([]byte(s[o:o+2]))), int(binary.LittleEndian.Uint16([]byte(s[o+2:o+6])))
		if o+keysz+valuesz > len(s) {
			return nil, fmt.Errorf("effect unmarshal: effect at byte %d too large", o)
		}
		e := Effect{s[o+6 : o+6+keysz], s[o+6+keysz : o+6+keysz+valuesz]}
		es = append(es, e)
		o += 6 + keysz + valuesz
		if e.Key <= lastKey {
			return nil, fmt.Errorf("effect unmarshal: %dth key not in sorted order", len(es)-1)
		}
		lastKey = e.Key
	}
	if o != len(s) {
		return nil, fmt.Errorf("effect unmarshal: incomplete last effect")
	}

	return es, nil
}
