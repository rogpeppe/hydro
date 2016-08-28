package hydroserver

import (
	"net/http"
	"net/http/httptest"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroconfig"
)

type configErrorSuite struct{}

var _ = gc.Suite(configErrorSuite{})

var serveConfigErrorTests = []struct {
	about        string
	err          error
	expectStatus int
	expect       string
}{{
	about:        "not parse error",
	err:          errgo.New("some error"),
	expectStatus: http.StatusBadRequest,
	expect:       `bad request \(POST /\): bad configuration: some error\n`,
}, {
	about: "one error",
	err: &hydroconfig.ConfigParseError{
		Config: "hello\ncruel\nworld",
		Errors: []hydroconfig.ParseError{{
			P0:      6,
			P1:      11,
			Message: "some message",
		}},
	},
	expectStatus: http.StatusBadRequest,
	expect:       `(?s).*<div>hello<br><span class="errorText">cruel<div class="toolTip">Some message<br></div></span><br>world</div>.*`,
}}

func (configErrorSuite) TestServeConfigError(c *gc.C) {
	for i, test := range serveConfigErrorTests {
		c.Logf("test %d: %s", i, test.about)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/", nil) // not actually used.
		serveConfigError(rec, req, test.err)
		c.Assert(string(rec.Body.Bytes()), gc.Matches, test.expect)
	}
}

var errorTextSegmentsTests = []struct {
	about  string
	err    *hydroconfig.ConfigParseError
	expect []segment
}{{
	about: "no errors",
	err: &hydroconfig.ConfigParseError{
		Config: "hello world",
	},
	expect: []segment{{
		Text: "hello world",
	}},
}, {
	about: "one error",
	err: &hydroconfig.ConfigParseError{
		Config: "hello cruel world",
		Errors: []hydroconfig.ParseError{{
			P0:      6,
			P1:      11,
			Message: "the world is not cruel",
		}},
	},
	expect: []segment{{
		Text: "hello ",
	}, {
		Text:   "cruel",
		Errors: []string{"the world is not cruel"},
	}, {
		Text: " world",
	}},
}, {
	about: "multiple errors",
	err: &hydroconfig.ConfigParseError{
		Config: "hello crueler world",
		Errors: []hydroconfig.ParseError{{
			P0:      0,
			P1:      19,
			Message: "the whole thing is bad",
		}, {
			// "cruel"
			P0:      6,
			P1:      11,
			Message: "the world is not cruel",
		}, {
			// "rueler"
			P0:      7,
			P1:      13,
			Message: "misspelling",
		}, {
			// "" (before "world"); will expand to "crueler world"
			P0:      14,
			P1:      14,
			Message: "empty",
		}},
	},
	expect: []segment{{
		Text:   "hello ",
		Errors: []string{"the whole thing is bad"},
	}, {
		Text: "c",
		Errors: []string{
			"the whole thing is bad",
			"the world is not cruel",
			"empty",
		},
	}, {
		Text: "ruel",
		Errors: []string{
			"the whole thing is bad",
			"the world is not cruel",
			"empty",
			"misspelling",
		},
	}, {
		Text: "er",
		Errors: []string{
			"the whole thing is bad",
			"empty",
			"misspelling",
		},
	}, {
		Text: " world",
		Errors: []string{
			"the whole thing is bad",
			"empty",
		},
	}},
}, {
	about: "with newlines, no errors",
	err: &hydroconfig.ConfigParseError{
		Config: "hello\nworld",
	},
	expect: []segment{{
		Text: "hello",
	}, {
		Text: "\n",
	}, {
		Text: "world",
	}},
}, {
	about: "with newlines and errors",
	err: &hydroconfig.ConfigParseError{
		Config: "hello\nworld",
		Errors: []hydroconfig.ParseError{{
			P0:      2,
			P1:      8,
			Message: "straddle two lines",
		}},
	},
	expect: []segment{{
		Text: "he",
	}, {
		Text:   "llo",
		Errors: []string{"straddle two lines"},
	}, {
		Text: "\n",
	}, {
		Text:   "wo",
		Errors: []string{"straddle two lines"},
	}, {
		Text: "rld",
	}},
}}

func (configErrorSuite) TestErrorTextSegments(c *gc.C) {
	for i, test := range errorTextSegmentsTests {
		c.Logf("test %d: %s", i, test.about)
		segs := errorTextSegments(test.err)
		c.Assert(segs, jc.DeepEquals, test.expect)
	}
}

var expandErrorTests = []struct {
	about  string
	config string
	err    *hydroconfig.ParseError
	expect *hydroconfig.ParseError
}{{
	about:  "non-zero non-whitespace - no expand needed",
	config: "hello world",
	err: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	about:  "zero-length at start",
	config: "hello world",
	err: &hydroconfig.ParseError{
		P0: 0,
		P1: 0,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	about:  "zero-length at end",
	config: "hello",
	err: &hydroconfig.ParseError{
		P0: 5,
		P1: 5,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	about:  "zero-length in middle",
	config: "all the space    in the world",
	err: &hydroconfig.ParseError{
		P0: 14,
		P1: 14,
	},
	expect: &hydroconfig.ParseError{
		P0: 8,
		P1: 19,
	},
}, {
	about:  "zero length at start of word",
	config: "hello crueler world",
	err: &hydroconfig.ParseError{
		P0: 14,
		P1: 14,
	},
	expect: &hydroconfig.ParseError{
		P0: 6,
		P1: 19,
	},
}, {
	about:  "zero length at end of word",
	config: "hello crueler world",
	err: &hydroconfig.ParseError{
		P0: 13,
		P1: 13,
	},
	expect: &hydroconfig.ParseError{
		P0: 6,
		P1: 19,
	},
}, {
	about:  "all white space",
	config: "all the space      in the world",
	err: &hydroconfig.ParseError{
		P0: 15,
		P1: 17,
	},
	expect: &hydroconfig.ParseError{
		P0: 8,
		P1: 21,
	},
}, {
	about:  "empty at start",
	config: "hello world",
	err: &hydroconfig.ParseError{
		P0: 0,
		P1: 0,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	about:  "empty at end",
	config: "hello world",
	err: &hydroconfig.ParseError{
		P0: 11,
		P1: 11,
	},
	expect: &hydroconfig.ParseError{
		P0: 6,
		P1: 11,
	},
}, {
	about:  "unicode space following word",
	config: "hello there\u00a0you",
	err: &hydroconfig.ParseError{
		P0: 6,
		P1: 6,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 11,
	},
}}

func (configErrorSuite) TestExpandError(c *gc.C) {
	for i, test := range expandErrorTests {
		c.Logf("test %d: %v", i, test.about)
		testErr := *test.err
		expandError(test.config, &testErr)
		c.Assert(&testErr, jc.DeepEquals, test.expect)
	}
}
