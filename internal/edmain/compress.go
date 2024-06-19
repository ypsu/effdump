package edmain

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"github.com/ypsu/effdump/internal/keyvalue"
	"github.com/ypsu/effdump/internal/textar"
)

const (
	maxEntries    int = 1e4
	maxTotalBytes int = 1e7
)

// Compress compresses `kvs` into a byte stream suitable for saving to disk.
// It's a gzip compressed textar with sepch used as the separator character.
// Returns an error if kvs is not sorted or encoding hit internal limits.
func Compress(kvs []keyvalue.KV, sepch byte, hash uint64) (data []byte, err error) {
	if len(kvs) > maxEntries {
		// This library wasn't designed for this huge size.
		return nil, fmt.Errorf("edmain marshal: effects count is %d K, limit is %d K", len(kvs)/1e3, maxEntries/1e3)
	}
	dumplen := 0
	for i := 1; i < len(kvs); i++ {
		dumplen += len(kvs[i].K) + len(kvs[i].V)
		if dumplen > maxTotalBytes {
			// This library wasn't designed for this huge size.
			return nil, fmt.Errorf("edmain marshal: effects size is %d MB, limit is %d MB", dumplen/1e6, maxTotalBytes/1e6)
		}
	}

	buf := &bytes.Buffer{}
	ar, w := textar.Format([]keyvalue.KV(kvs), sepch), gzip.NewWriter(buf)
	w.Header.Comment = fmt.Sprintf("effdump %d %d %016x", len(kvs), len(ar), hash)
	_, err = io.Copy(w, strings.NewReader(ar))
	if err != nil {
		return nil, fmt.Errorf("edmain/compress: %v", err)
	}
	w.Close()
	return buf.Bytes(), nil
}

// Uncompress decodes a compressed textar stream.
// See Marshal() for info about the format.
// The function is safe: it won't eat up all memory on adversial input or on a huge effdump.
// It only accepts effdumps within the limits.
func Uncompress(data []byte) ([]keyvalue.KV, error) {
	r, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("edmain/init decompressor: %v", err)
	}
	var entriesCount, textarLen int
	fmt.Sscanf(r.Header.Comment, "effdump %d %d", &entriesCount, &textarLen)
	if entriesCount > maxEntries || textarLen > maxTotalBytes {
		return nil, fmt.Errorf("edmain/header check: effdump too large: entriesK=%d > %d or totalMB=%d > %d", entriesCount/1e3, maxEntries/1e3, textarLen/1e6, maxTotalBytes/1e6)
	}
	w, kvs := &strings.Builder{}, make([]keyvalue.KV, 0, entriesCount)
	w.Grow(textarLen)
	limr := &io.LimitedReader{r, int64(maxTotalBytes + 1)}
	if _, err := io.Copy(w, limr); err != nil {
		return nil, fmt.Errorf("edmain/decompress: %v", err)
	}
	if limr.N == 0 {
		return nil, fmt.Errorf("edmain/decompress limit: decompress reached the limit of %d MB", maxTotalBytes/1e6)
	}
	return textar.Parse(kvs, w.String()), nil
}

// PeekHash returns the hash stored in the gzip header.
func PeekHash(f io.Reader) uint64 {
	r, err := gzip.NewReader(f)
	if err != nil {
		return 0
	}
	var entriesCount, textarLen int
	var hash uint64
	fmt.Sscanf(r.Header.Comment, "effdump %d %d %x", &entriesCount, &textarLen, &hash)
	return hash
}
