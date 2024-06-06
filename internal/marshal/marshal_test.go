package marshal_test

import (
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
