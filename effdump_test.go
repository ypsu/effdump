package effdump

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ypsu/effdump/internal/marshal"
)

func runRun(d *Dump) string {
	w := &strings.Builder{}
	run = func(name string, es marshal.Entries) {
		for _, e := range es {
			fmt.Fprintf(w, "%s/%s: %q\n", name, e.Key, e.Value)
		}
	}
	d.Run("testmap")
	return w.String()
}

func TestInt(t *testing.T) {
	d := &Dump{}
	AddMap(d, map[string]int{
		"apple":     5,
		"banana":    6,
		"cranberry": 9,
		"date":      4,
	})
	want := `testmap/apple: "5"
testmap/banana: "6"
testmap/cranberry: "9"
testmap/date: "4"
`
	if got := runRun(d); got != want {
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
	d := &Dump{}
	AddMap(d, map[humanint][]byte{
		5: []byte("apple"),
		6: []byte("banana"),
		9: []byte("cranberry"),
		4: []byte("date"),
	})
	want := `testmap/five: "apple"
testmap/four: "date"
testmap/nine: "cranberry"
testmap/six: "banana"
`
	if got := runRun(d); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}

func TestSlice(t *testing.T) {
	d := &Dump{}
	AddMap(d, map[byte][]string{
		'a': []string{"apple", "banana"},
		'c': []string{"cranberry", "date", "eggplant"},
		'f': []string{"fig", "kiwi", "lemon", "mango"},
	})
	want := `testmap/102: "[fig kiwi lemon mango]"
testmap/97: "[apple banana]"
testmap/99: "[cranberry date eggplant]"
`
	if got := runRun(d); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}

func TestMap(t *testing.T) {
	d := &Dump{}
	AddMap(d, map[string]map[byte]string{
		"tükör":   map[byte]string{'a': "apple", 'b': "banana"},
		"fúrógép": map[byte]string{'c': "cranberry", 'd': "date", 'e': "eggplant"},
	})
	want := `testmap/fúrógép: "map[99:cranberry 100:date 101:eggplant]"
testmap/tükör: "map[97:apple 98:banana]"
`
	if got := runRun(d); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}

func TestMultiAdd(t *testing.T) {
	d := &Dump{}
	d.Add(1, "apple")
	d.Add("list", []int{1, 2, 3})
	AddMap(d, map[string]int{"two": 2, "four": 4, "six": 6})
	d.Add('x', "banana")
	want := `testmap/1: "apple"
testmap/120: "banana"
testmap/four: "4"
testmap/list: "[1 2 3]"
testmap/six: "6"
testmap/two: "2"
`
	if got := runRun(d); got != want {
		t.Errorf("Diff in testmap data, got:\n%s", got)
	}
}
