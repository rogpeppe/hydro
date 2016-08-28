package hydroconfig

import (
	"strings"
	"unicode"
)

type text struct {
	s      string
	p0, p1 int
}

func newText(s string) text {
	return text{
		s:  s,
		p1: len(s),
	}
}

func (t text) slice(p0, p1 int) text {
	return text{
		s:  t.s[p0:p1],
		p0: t.p0 + p0,
		p1: t.p0 + p1,
	}
}

func (t text) eqFold(s string) bool {
	return strings.EqualFold(t.s, s)
}

// word returns the non-whitespace run of runes in in t starting after any leading
// white space and the rest of the text following that.
func (t text) word() (text, text) {
	found := false
	start := 0
	for i, c := range t.s {
		if !unicode.IsSpace(c) {
			found = true
			start = i
			break
		}
	}
	if !found {
		return t.slice(0, 0), t.slice(len(t.s), len(t.s))
	}
	t = t.slice(start, len(t.s))
	end := len(t.s)
	for i, c := range t.s {
		if unicode.IsSpace(c) {
			end = i
			break
		}
	}
	return t.slice(0, end), t.slice(end, len(t.s))
}

func (t text) line() (text, text) {
	i := strings.Index(t.s, "\n")
	if i == -1 {
		return t, text{
			p0: len(t.s),
			p1: len(t.s),
		}
	}
	return text{
			s:  t.s[0:i],
			p0: t.p0,
			p1: t.p0 + i,
		}, text{
			s:  t.s[i+1:],
			p0: t.p0 + i + 1,
			p1: t.p1,
		}
}

func (t text) trimSpace() text {
	for i, c := range t.s {
		if !unicode.IsSpace(c) {
			t = t.slice(i, len(t.s))
			break
		}
	}
	t.s = strings.TrimRightFunc(t.s, unicode.IsSpace)
	t.p1 = t.p0 + len(t.s)
	return t
}

func (t text) trimWord(p string) (text, bool) {
	tw, t1 := t.word()
	if tw.eqFold(p) {
		return t1, true
	}
	return t, false
}

func (t text) trimPrefix(p string) (text, bool) {
	t0 := t
	pfields := strings.Fields(p)
	for {
		if len(pfields) == 0 {
			return t, true
		}
		var tw text
		tw, t = t.word()
		if len(tw.s) == 0 {
			return t, false
		}
		if !tw.eqFold(pfields[0]) {
			return t0, false
		}
		pfields = pfields[1:]
	}
}
