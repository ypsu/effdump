package marshal_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ypsu/effdump/internal/marshal"
)

func TestHeader(t *testing.T) {
	es := marshal.Entries{{"somekey", "somevalue"}}
	data, err := es.MarshalBinary()
	if err != nil {
		t.Errorf("MarshalBinary() = %v, want no error.", err)
	}
	if !strings.HasPrefix(string(data), "effdump0") {
		t.Errorf(`MarshalBinary() = %q, want "effdump0".`, string(data)[:8])
	}
}

func TestLimit(t *testing.T) {
	es := marshal.Entries{{}}
	es[0].Key = strings.Repeat("x", 1<<16)
	_, err := es.MarshalBinary()
	if err == nil {
		t.Errorf("MarshalBinary() = nil, want error.")
	}
}

func TestUnmarshal(t *testing.T) {
	src := make(marshal.Entries, 0, 7)
	for i := 0; i < 7; i++ {
		src = append(src, marshal.Entry{fmt.Sprint(i), strings.Repeat("x", i)})
	}
	data, err := src.MarshalBinary()
	if err != nil {
		t.Errorf("MarshalBinary() = %v, want no error.", err)
	}

	var dst marshal.Entries
	err = dst.UnmarshalBinary(data)
	if err != nil {
		t.Errorf("UnmarshalBinary() = %v, want no error.", err)
	}
	if !slices.Equal(src, dst) {
		t.Errorf("UnmarshalBinary():\ngot: %v\nwant: %v\n", dst, src)
	}
}
