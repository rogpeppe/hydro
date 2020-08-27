package hydroserver

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroconfig"
)

type errorText struct {
	Segments []segment
}

type segment struct {
	Text   string
	Errors []string
}

var configErrorTempl = newTemplate(`
<html>
	<head>
		<title>Error in configuration</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<link rel="stylesheet" href="/common.css">
	</head>
</head>
<body>
<h3>Errors found in configuration</h3>
<p>Hover mouse over underlined text to show the errors.
</p>
<div>
	{{- range .Segments -}}
		{{- if eq .Text "\n" -}}<br>
		{{- else -}}
			{{- if eq (len .Errors) 0 -}}
				{{.Text|nbsp}}
			{{- else -}}
				<span class="errorText">
				{{- if eq .Text "\n" -}}<br>{{else}}{{.Text|nbsp}}{{end -}}
				<div class="toolTip">
					{{- range .Errors -}}
						{{- . | capitalize -}}<br>
					{{- end -}}
				</div></span>
			{{- end -}}
		{{- end -}}
	{{- end -}}
</div>
</body>
</html>
`)

func serveConfigError(w http.ResponseWriter, req *http.Request, err error) {
	cfgErr, ok := errgo.Cause(err).(*hydroconfig.ConfigParseError)
	if !ok {
		badRequest(w, req, errgo.Newf("bad configuration: %v", err))
		return
	}
	segs := errorTextSegments(cfgErr)
	var b bytes.Buffer
	if err := configErrorTempl.Execute(&b, &errorText{
		Segments: segs,
	}); err != nil {
		log.Printf("template execution failed: %v", err)
		http.Error(w, fmt.Sprintf("template execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	w.Write(b.Bytes())
}

// transition represents a start or a stop of an error message in the configuration text.
type transition struct {
	errIndex int
	pos      int
	start    bool
	text     string
}

func errorTextSegments(parseErr *hydroconfig.ConfigParseError) []segment {
	// Expand the range of any errors that are going to become invisible.
	expandErrors(parseErr)

	// Set up all the transitions. A transition happens when an error that applies
	// to a given range of text starts or stops. We order all the transitions
	// and then spit out a segment every time there is a transition.
	transitions := make(transitions, 0, len(parseErr.Errors)*2+2)
	for i, e := range parseErr.Errors {
		transitions = append(transitions, transition{
			errIndex: i,
			pos:      e.P0,
			start:    true,
			text:     e.Message,
		}, transition{
			errIndex: i,
			pos:      e.P1,
		})
	}
	sort.Sort(transitions)

	segments := make([]segment, 0, len(parseErr.Errors)*2)
	putRange := func(p0, p1 int, errors []string) {
		if p0 == p1 {
			return
		}
		segments = append(segments, segment{
			Text:   parseErr.Config[p0:p1],
			Errors: errors,
		})
	}

	current := make(map[int]transition)
	prev := 0
	for _, t := range transitions {
		putRange(prev, t.pos, allErrors(current))
		if t.start {
			current[t.errIndex] = t
		} else {
			delete(current, t.errIndex)
		}
		prev = t.pos
	}
	if len(current) > 0 {
		panic("unexpected errors still in stack")
	}
	putRange(prev, len(parseErr.Config), nil)

	return addLineBreaks(segments)
}

// expandErrors expands any errors that will be hard to see
// to include some adjacent text too.
func expandErrors(err *hydroconfig.ConfigParseError) {
	for i := range err.Errors {
		expandError(err.Config, &err.Errors[i])
	}
}

// expandError expands an error to include the words either side of it.
func expandError(s string, err *hydroconfig.ParseError) {
	if !needsExpand(s, err) {
		return
	}
	p0 := strings.LastIndexFunc(s[0:err.P0], notSpace)
	if p0 != -1 {
		p0 = strings.LastIndexFunc(s[0:p0], unicode.IsSpace)
		if p0 == -1 {
			p0 = 0
		} else {
			// We're on the last space; advance to rune after it
			// so that we don't include the space.
			_, size := utf8.DecodeRuneInString(s[p0:])
			p0 += size
		}
	} else {
		p0 = 0
	}

	p1 := strings.IndexFunc(s[err.P1:], notSpace)
	if p1 != -1 {
		p1 += err.P1
		p1i := strings.IndexFunc(s[p1:], unicode.IsSpace)
		if p1i == -1 {
			p1 = len(s)
		} else {
			p1 += p1i
		}
	} else {
		p1 = len(s)
	}
	err.P0, err.P1 = p0, p1
}

func notSpace(r rune) bool {
	return !unicode.IsSpace(r)
}

func needsExpand(s string, err *hydroconfig.ParseError) bool {
	return err.P0 == err.P1 || strings.TrimSpace(s[err.P0:err.P1]) == ""
}

// addLineBreaks adds a segment for each new line in every
// segment, so that we can add <br>s in the output HTML
// for those.
func addLineBreaks(ss []segment) []segment {
	newss := make([]segment, 0, len(ss))
	for _, seg := range ss {
		lines := strings.Split(seg.Text, "\n")
		newss = append(newss, segment{
			Text:   lines[0],
			Errors: seg.Errors,
		})
		for _, line := range lines[1:] {
			newss = append(newss, segment{
				Text: "\n",
			}, segment{
				Text:   line,
				Errors: seg.Errors,
			})
		}
	}
	return newss
}

func allErrors(m map[int]transition) []string {
	if len(m) == 0 {
		return nil
	}
	ts := make(transitions, 0, len(m))
	for _, t := range m {
		ts = append(ts, t)
	}
	sort.Sort(ts)
	errors := make([]string, len(ts))
	for i, t := range ts {
		errors[i] = t.text
	}
	return errors
}

type transitions []transition

func (ts transitions) Less(i, j int) bool {
	t0, t1 := ts[i], ts[j]
	if t0.pos != t1.pos {
		return t0.pos < t1.pos
	}
	if t0.errIndex != t1.errIndex {
		return t0.errIndex < t1.errIndex
	}
	return t0.start
}

func (ts transitions) Swap(i, j int) {
	ts[i], ts[j] = ts[j], ts[i]
}

func (ts transitions) Len() int {
	return len(ts)
}
