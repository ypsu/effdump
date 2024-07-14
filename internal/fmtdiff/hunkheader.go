package fmtdiff

import (
	"strings"
	"unicode"
)

// hunkheader is for finding an appropriate header for a zipped hunk.
// It tries to pick the non-empty line with the lowest indent.
// It ignores lines that don't start with a letter.
// It picks the latest line with the lowest index except when they are consecutive, then it prefers to pick the first from that block.
// In case of paragraphs this should pick the first line.
// It's unclear how good this heuristic is but it's a simple placeholder logic until something more decent comes along.
type hunkheader struct {
	indent int
	line   string
	inrun  bool
}

func countIndent(s string) int {
	if strings.TrimSpace(s) == "" {
		return 1 << 30
	}
	indent := 0
	for strings.HasPrefix(s, " ") || strings.HasPrefix(s, "\t") {
		s, indent = s[1:], indent+1
	}
	return indent
}

func (h *hunkheader) improve(s string) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) <= 2 || !(unicode.IsLetter(rune(trimmed[0])) || unicode.IsLetter(rune(trimmed[1]))) {
		h.inrun = false
		return
	}
	indent := countIndent(s)
	if h.line != "" && indent > h.indent {
		h.inrun = false
		return
	}
	if indent == h.indent && h.inrun {
		return
	}
	h.indent, h.line, h.inrun = indent, s, true
}

// header returns the hunk header but only if the first non-empty line in xs has higher indent.
func (h *hunkheader) header(xs []string) string {
	if h.line == "" {
		return ""
	}
	for _, s := range xs {
		if strings.TrimSpace(s) == "" {
			continue
		}
		indent := countIndent(s)
		if indent > h.indent || indent == h.indent && h.inrun {
			return " " + strings.TrimSpace(h.line)
		}
		return ""
	}
	return ""
}
