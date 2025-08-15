package jaws

import (
	"fmt"
	"html"
	"html/template"
)

type htmlGetter struct{ v template.HTML }

func (g htmlGetter) JawsGetHTML(e *Element) template.HTML {
	return g.v
}

func (g htmlGetter) JawsGetTag(rq *Request) any {
	return nil
}

type htmlStringerGetter struct{ sg fmt.Stringer }

func (g htmlStringerGetter) JawsGetHTML(e *Element) template.HTML {
	return template.HTML(html.EscapeString(g.sg.String())) // #nosec G203
}

func (g htmlStringerGetter) JawsGetTag(rq *Request) any {
	return g.sg
}

type htmlGetterString struct{ sg Getter[string] }

func (g htmlGetterString) JawsGetHTML(e *Element) template.HTML {
	return template.HTML(html.EscapeString(g.sg.JawsGet(e))) // #nosec G203
}

func (g htmlGetterString) JawsGetTag(rq *Request) any {
	return g.sg
}

// MakeHTMLGetter returns a HTMLGetter for v.
//
// Depending on the type of v, we return:
//
//   - jaws.HTMLGetter: `JawsGetHTML(e *Element) template.HTML` to be used as-is.
//   - jaws.Getter[string]: `JawsGet(elem *Element) string` that will be escaped using `html.EscapeString`.
//   - jaws.AnyGetter: `JawsGetAny(elem *Element) any` that will be rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.
//   - fmt.Stringer: `String() string` that will be escaped using `html.EscapeString`.
//   - a static `template.HTML` or `string` to be used as-is with no HTML escaping.
//   - everything else is rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.
func MakeHTMLGetter(v any) HTMLGetter {
	switch v := v.(type) {
	case HTMLGetter:
		return v
	case template.HTML:
		return htmlGetter{v}
	case Getter[string]:
		return htmlGetterString{v}
	case fmt.Stringer:
		return htmlStringerGetter{v}
	case string:
		return htmlGetter{template.HTML(v)} // #nosec G203
	case Formatter:
		return htmlGetterString{v.Format("%v")}
	default:
		return htmlGetter{template.HTML(html.EscapeString(fmt.Sprint(v)))} // #nosec G203
	}
}
