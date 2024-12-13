package jaws

import (
	"fmt"
	"html"
	"html/template"
)

type htmlGetter struct{ v template.HTML }

func (g htmlGetter) JawsGetHtml(e *Element) template.HTML {
	return g.v
}

func (g htmlGetter) JawsGetTag(rq *Request) any {
	return nil
}

type htmlGetterStringGetter struct{ sg StringGetter }

func (g htmlGetterStringGetter) JawsGetHtml(e *Element) template.HTML {
	return template.HTML(html.EscapeString(g.sg.JawsGetString(e))) // #nosec G203
}

func (g htmlGetterStringGetter) JawsGetTag(rq *Request) any {
	return g.sg
}

type htmlGetterHTML struct{ sg Getter[template.HTML] }

func (g htmlGetterHTML) JawsGetHtml(e *Element) template.HTML {
	return g.sg.JawsGet(e)
}

func (g htmlGetterHTML) JawsGetTag(rq *Request) any {
	return g.sg
}

type htmlGetterString struct{ sg Getter[string] }

func (g htmlGetterString) JawsGetHtml(e *Element) template.HTML {
	return template.HTML(html.EscapeString(g.sg.JawsGet(e))) // #nosec G203
}

func (g htmlGetterString) JawsGetTag(rq *Request) any {
	return g.sg
}

type htmlGetterAny struct{ ag AnyGetter }

func (g htmlGetterAny) JawsGetHtml(e *Element) template.HTML {
	s := fmt.Sprint(g.ag.JawsGetAny(e))
	return template.HTML(html.EscapeString(s)) // #nosec G203
}

func (g htmlGetterAny) JawsGetTag(rq *Request) any {
	return g.ag
}

// MakeHtmlGetter returns a HtmlGetter for v.
//
// Depending on the type of v, we return:
//
//   - jaws.HtmlGetter: `JawsGetHtml(e *Element) template.HTML` to be used as-is.
//   - jaws.Getter[template.HTML]: `JawsGet(elem *Element) template.HTML` to be used as-is.
//   - jaws.StringGetter: `JawsGetString(e *Element) string` that will be escaped using `html.EscapeString`.
//   - jaws.Getter[string]: `JawsGet(elem *Element) string` that will be escaped using `html.EscapeString`.
//   - jaws.AnyGetter: `JawsGetAny(elem *Element) any` that will be rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.
//   - fmt.Stringer: `String() string` that will be escaped using `html.EscapeString`.
//   - a static `template.HTML` or `string` to be used as-is with no HTML escaping.
//   - everything else is rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.
func MakeHtmlGetter(v any) HtmlGetter {
	switch v := v.(type) {
	case HtmlGetter:
		return v
	case Getter[template.HTML]:
		return htmlGetterHTML{v}
	case StringGetter:
		return htmlGetterStringGetter{v}
	case Getter[string]:
		return htmlGetterString{v}
	case AnyGetter:
		return htmlGetterAny{v}
	case fmt.Stringer:
		return htmlGetterStringGetter{stringerGetter{v}}
	case template.HTML:
		return htmlGetter{v}
	case string:
		return htmlGetter{template.HTML(v)} // #nosec G203
	default:
		return htmlGetter{template.HTML(html.EscapeString(fmt.Sprint(v)))} // #nosec G203
	}
}
