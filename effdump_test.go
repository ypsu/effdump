package effdump

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ypsu/effdump/internal/marshal"
)

func runRun[M ~map[K]V, K comparable, V any](m M) string {
	w := &strings.Builder{}
	run = func(name string, es marshal.Entries) {
		for _, e := range es {
			fmt.Fprintf(w, "%s/%s: %q\n", name, e.Key, e.Value)
		}
	}
	Run("testmap", m)
	return w.String()
}

func TestInt(t *testing.T) {
	m := map[string]int{
		"apple":     5,
		"banana":    6,
		"cranberry": 9,
		"date":      4,
	}
	want := `testmap/apple: "5"
testmap/banana: "6"
testmap/cranberry: "9"
testmap/date: "4"
`
	if got := runRun(m); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}

type humanint int

func (i humanint) String() string {
	return []string{
		"zero", "one", "two", "three", "four",
		"five", "six", "seven", "eight", "nine",
	}[i]
}

func TestStringer(t *testing.T) {
	m := map[humanint][]byte{
		5: []byte("apple"),
		6: []byte("banana"),
		9: []byte("cranberry"),
		4: []byte("date"),
	}
	want := `testmap/five: "apple"
testmap/four: "date"
testmap/nine: "cranberry"
testmap/six: "banana"
`
	if got := runRun(m); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}

func TestSlice(t *testing.T) {
	m := map[byte][]string{
		'a': []string{"apple", "banana"},
		'c': []string{"cranberry", "date", "eggplant"},
		'f': []string{"fig", "kiwi", "lemon", "mango"},
	}
	want := `testmap/102: "[fig kiwi lemon mango]"
testmap/97: "[apple banana]"
testmap/99: "[cranberry date eggplant]"
`
	if got := runRun(m); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}

func TestMap(t *testing.T) {
	m := map[string]map[byte]string{
		"tükör":   map[byte]string{'a': "apple", 'b': "banana"},
		"fúrógép": map[byte]string{'c': "cranberry", 'd': "date", 'e': "eggplant"},
	}
	want := `testmap/fúrógép: "map[99:cranberry 100:date 101:eggplant]"
testmap/tükör: "map[97:apple 98:banana]"
`
	if got := runRun(m); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}
