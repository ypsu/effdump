package textar_test

import (
	"slices"
	"testing"

	"github.com/ypsu/effdump/internal/effect"
	"github.com/ypsu/effdump/internal/textar"
)

func TestAr(t *testing.T) {
	src := []effect.Effect{
		{"hello", "world"},
		{"a newline\nname", "multiple\nlines\nin value too\n"},
		{"", "this has no name\n--- and has 3 dashes too\n"},
		{"last", "entry"},
	}
	ar := textar.Format(src)
	dst := textar.Parse(ar)
	if !slices.Equal(src, dst) {
		t.Errorf("Error self-decoding, archive:\n%s\ndecoded into this:\n%q", ar, dst)
		for i := 0; i < min(len(src), len(dst)); i++ {
			if dst[i] != src[i] {
				t.Logf("First difference at index %d: %q != %q", i, dst[i], src[i])
				break
			}
		}
	}
}
