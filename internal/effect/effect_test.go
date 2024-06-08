package effect_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ypsu/effdump/internal/effect"
)

func TestHeader(t *testing.T) {
	es := []effect.Effect{{"somekey", "somevalue"}}
	data, err := effect.Marshal(es)
	if err != nil {
		t.Errorf("Marshal() = %v, want no error.", err)
	}
	if !strings.HasPrefix(string(data), "effdump0") {
		t.Errorf(`Marshal() = %q, want "effdump0".`, string(data)[:8])
	}
}

func TestLimit(t *testing.T) {
	es := []effect.Effect{{}}
	es[0].Key = strings.Repeat("x", 1<<16)
	_, err := effect.Marshal(es)
	if err == nil {
		t.Errorf("Marshal() = nil, want error.")
	}
}

func TestUnmarshal(t *testing.T) {
	src := make([]effect.Effect, 0, 7)
	for i := 0; i < 7; i++ {
		src = append(src, effect.Effect{fmt.Sprint(i), strings.Repeat("x", i)})
	}
	data, err := effect.Marshal(src)
	if err != nil {
		t.Errorf("Marshal() = %v, want no error.", err)
	}

	dst, err := effect.Unmarshal(data)
	if err != nil {
		t.Errorf("Unmarshal() = %v, want no error.", err)
	}
	if !slices.Equal(src, dst) {
		t.Errorf("Unmarshal():\ngot: %v\nwant: %v\n", dst, src)
	}
}
