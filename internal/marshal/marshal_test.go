package marshal_test

import (
	"strings"
	"testing"

	"github.com/ypsu/effdump/internal/marshal"
)

func TestHeader(t *testing.T) {
	es := marshal.Entries{{"somekey", "somevalue"}}
	data, _ := es.MarshalBinary()
	if !strings.HasPrefix(string(data), "effdump0") {
		t.Error("effdump0 header missing.")
	}
}
