package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync/atomic"
)

type HtmlGetter interface {
	JawsGetHtml(e *Element) template.HTML
}

type htmlGetter struct{ v template.HTML }

func (g htmlGetter) JawsGetHtml(e *Element) template.HTML {
	return g.v
}

func (g htmlGetter) JawsGetTag(rq *Request) any {
	return nil
}

type htmlStringGetter struct{ sg StringGetter }

func (g htmlStringGetter) JawsGetHtml(e *Element) template.HTML {
	return template.HTML(html.EscapeString(g.sg.JawsGetString(e))) // #nosec G203
}

func (g htmlStringGetter) JawsGetTag(rq *Request) any {
	return g.sg
}

func makeHtmlGetter(v any) HtmlGetter {
	switch v := v.(type) {
	case HtmlGetter:
		return v
	case StringGetter:
		return htmlStringGetter{v}
	case *atomic.Value:
		return atomicSetter{v}
	case template.HTML:
		return htmlGetter{v}
	case string:
		h := template.HTML(v) // #nosec G203
		return htmlGetter{h}
	}
	panic(fmt.Errorf("expected string, jaws.StringGetter or jaws.HtmlGetter, not %T", v))
}
