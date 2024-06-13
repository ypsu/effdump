package edmain_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ypsu/effdump/internal/edmain"
	"github.com/ypsu/effdump/internal/keyvalue"
)

func TestUncompress(t *testing.T) {
	src := make([]keyvalue.KV, 0, 7)
	for i := 0; i < 7; i++ {
		src = append(src, keyvalue.KV{fmt.Sprint(i), strings.Repeat("x", i)})
	}
	data, err := edmain.Compress(src, '=')
	if err != nil {
		t.Errorf("Compress() = %v, want no error.", err)
	}

	dst, err := edmain.Uncompress(data)
	if err != nil {
		t.Errorf("Uncompress() = %v, want no error.", err)
	}
	if !slices.Equal(src, dst) {
		t.Errorf("Uncompress():\ngot: %v\nwant: %v\n", dst, src)
	}
}
