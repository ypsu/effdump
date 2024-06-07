package effdump

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ypsu/effdump/internal/marshal"
)

func TestRun(t *testing.T) {
	m := map[string]int{
		"apple":     5,
		"banana":    6,
		"cranberry": 9,
		"date":      4,
	}

	w := &strings.Builder{}
	run = func(name string, es marshal.Entries) {
		for _, e := range es {
			fmt.Fprintf(w, "%s/%s: %q\n", name, e.Key, e.Value)
		}
	}

	Run("testmap", m)

	want := `testmap/apple: "5"
testmap/banana: "6"
testmap/cranberry: "9"
testmap/date: "4"
`
	if got := w.String(); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}
