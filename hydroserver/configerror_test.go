package hydroserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroconfig"
)

var serveConfigErrorTests = []struct {
	testName     string
	err          error
	expectStatus int
	expect       string
}{{
	testName:     "not-parse-error",
	err:          errgo.New("some error"),
	expectStatus: http.StatusBadRequest,
	expect:       `bad request \(POST /\): bad configuration: some error\n`,
}, {
	testName: "one-error",
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

func TestServeConfigError(t *testing.T) {
	c := qt.New(t)
	for _, test := range serveConfigErrorTests {
		c.Run(test.testName, func(c *qt.C) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", nil) // not actually used.
			serveConfigError(rec, req, test.err)
			c.Assert(string(rec.Body.Bytes()), qt.Matches, test.expect)
		})
	}
}

var errorTextSegmentsTests = []struct {
	testName string
	err      *hydroconfig.ConfigParseError
	expect   []segment
}{{
	testName: "no-errors",
	err: &hydroconfig.ConfigParseError{
		Config: "hello world",
	},
	expect: []segment{{
		Text: "hello world",
	}},
}, {
	testName: "one-error",
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
	testName: "multiple-errors",
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
	testName: "with-newlines,-no-errors",
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
	testName: "with-newlines-and-errors",
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

func TestErrorTextSegments(t *testing.T) {
	c := qt.New(t)

	for _, test := range errorTextSegmentsTests {
		c.Run(test.testName, func(c *qt.C) {
			segs := errorTextSegments(test.err)
			c.Assert(segs, qt.DeepEquals, test.expect)
		})
	}
}

var expandErrorTests = []struct {
	testName string
	config   string
	err      *hydroconfig.ParseError
	expect   *hydroconfig.ParseError
}{{
	testName: "non-zero-non-whitespace---no-expand-needed",
	config:   "hello world",
	err: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	testName: "zero-length-at-start",
	config:   "hello world",
	err: &hydroconfig.ParseError{
		P0: 0,
		P1: 0,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	testName: "zero-length-at-end",
	config:   "hello",
	err: &hydroconfig.ParseError{
		P0: 5,
		P1: 5,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	testName: "zero-length-in-middle",
	config:   "all the space    in the world",
	err: &hydroconfig.ParseError{
		P0: 14,
		P1: 14,
	},
	expect: &hydroconfig.ParseError{
		P0: 8,
		P1: 19,
	},
}, {
	testName: "zero-length-at-start-of-word",
	config:   "hello crueler world",
	err: &hydroconfig.ParseError{
		P0: 14,
		P1: 14,
	},
	expect: &hydroconfig.ParseError{
		P0: 6,
		P1: 19,
	},
}, {
	testName: "zero-length-at-end-of-word",
	config:   "hello crueler world",
	err: &hydroconfig.ParseError{
		P0: 13,
		P1: 13,
	},
	expect: &hydroconfig.ParseError{
		P0: 6,
		P1: 19,
	},
}, {
	testName: "all-white-space",
	config:   "all the space      in the world",
	err: &hydroconfig.ParseError{
		P0: 15,
		P1: 17,
	},
	expect: &hydroconfig.ParseError{
		P0: 8,
		P1: 21,
	},
}, {
	testName: "empty-at-start",
	config:   "hello world",
	err: &hydroconfig.ParseError{
		P0: 0,
		P1: 0,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 5,
	},
}, {
	testName: "empty-at-end",
	config:   "hello world",
	err: &hydroconfig.ParseError{
		P0: 11,
		P1: 11,
	},
	expect: &hydroconfig.ParseError{
		P0: 6,
		P1: 11,
	},
}, {
	testName: "unicode-space-following-word",
	config:   "hello there\u00a0you",
	err: &hydroconfig.ParseError{
		P0: 6,
		P1: 6,
	},
	expect: &hydroconfig.ParseError{
		P0: 0,
		P1: 11,
	},
}}

func TestExpandError(t *testing.T) {
	c := qt.New(t)
	for _, test := range expandErrorTests {
		c.Run(test.testName, func(c *qt.C) {
			testErr := *test.err
			expandError(test.config, &testErr)
			c.Assert(&testErr, qt.DeepEquals, test.expect)
		})
	}
}
