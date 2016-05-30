package hydroconfig

import (
	jc "github.com/juju/testing/checkers"

	gc "gopkg.in/check.v1"
)

var _ = gc.Suite(&textSuite{})

type textSuite struct{}

var wordTests = []struct {
	t          text
	expect     text
	expectRest text
}{{
	t:          newText(""),
	expect:     text{},
	expectRest: text{},
}, {
	t:      newText(" \t"),
	expect: text{},
	expectRest: text{
		p0: 2,
		p1: 2,
	},
}, {
	t: newText("x"),
	expect: text{
		s:  "x",
		p0: 0,
		p1: 1,
	},
	expectRest: text{
		p0: 1,
		p1: 1,
	},
}, {
	t: newText("hello"),
	expect: text{
		s:  "hello",
		p0: 0,
		p1: 5,
	},
	expectRest: text{
		p0: 5,
		p1: 5,
	},
}, {
	t: newText("hello world"),
	expect: text{
		s:  "hello",
		p0: 0,
		p1: 5,
	},
	expectRest: text{
		s:  " world",
		p0: 5,
		p1: 11,
	},
}, {
	t: newText("  hello world"),
	expect: text{
		s:  "hello",
		p0: 2,
		p1: 7,
	},
	expectRest: text{
		s:  " world",
		p0: 7,
		p1: 13,
	},
}, {
	t: text{
		s:  " hello world",
		p0: 10,
		p1: 22,
	},
	expect: text{
		s:  "hello",
		p0: 11,
		p1: 16,
	},
	expectRest: text{
		s:  " world",
		p0: 16,
		p1: 22,
	},
}}

func (*textSuite) TestWord(c *gc.C) {
	for i, test := range wordTests {
		c.Logf("test %d; %q", i, test.t.s)
		word, rest := test.t.word()
		c.Check(word, jc.DeepEquals, test.expect)
		c.Check(rest, jc.DeepEquals, test.expectRest)
	}
}

var trimPrefixTests = []struct {
	t        text
	prefix   string
	expect   text
	expectOK bool
}{{
	t:      newText("hello world"),
	prefix: "hello",
	expect: text{
		s:  " world",
		p0: 5,
		p1: 11,
	},
	expectOK: true,
}, {
	t:      newText("   Hello    thERE you"),
	prefix: " hello  There ",
	expect: text{
		s:  " you",
		p0: 17,
		p1: 21,
	},
	expectOK: true,
}, {
	t:        newText("   Hewllo"),
	prefix:   " hello  There ",
	expect:   text{},
	expectOK: false,
}}

func (*textSuite) TestTrimPrefix(c *gc.C) {
	for i, test := range trimPrefixTests {
		c.Logf("test %d; %q", i, test.t.s)
		rest, ok := test.t.trimPrefix(test.prefix)
		c.Check(rest, jc.DeepEquals, test.expect)
		c.Check(ok, gc.Equals, test.expectOK)
	}
}
