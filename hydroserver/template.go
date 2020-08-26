package hydroserver

import (
	"fmt"
	"html/template"
	"strings"
	"unicode/utf8"
)

var tmplFuncs = template.FuncMap{
	"nbsp": func(s string) string {
		return strings.Replace(s, " ", "\u00a0", -1)
	},
	"capitalize": func(s string) string {
		_, n := utf8.DecodeRuneInString(s)
		if u := strings.ToUpper(s[0:n]); u != s[0:n] {
			return u + s[n:]
		}
		return s
	},
	"joinSp": func(ss []string) string {
		return strings.Join(ss, " ")
	},
	"mul": func(f1, f2 float64) float64 {
		return f1 * f2
	},
	"kWh": func(f float64) string {
		return fmt.Sprintf("%.3fkWh", f/1000)
	},
}

func newTemplate(s string) *template.Template {
	return template.Must(template.New("").Funcs(tmplFuncs).Parse(s))
}
