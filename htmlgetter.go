package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync/atomic"
)

type HtmlGetter interface {
	JawsGetHtml(rq *Element) template.HTML
}

type htmlGetter struct{ v template.HTML }

func (g htmlGetter) JawsGetHtml(e *Element) template.HTML {
	return g.v
}

func (g htmlGetter) JawsGetTag(rq *Request) any {
	return nil
}

type htmlStringGetter struct{ v StringSetter }

func (g htmlStringGetter) JawsGetHtml(e *Element) template.HTML {
	return template.HTML(html.EscapeString(g.v.JawsGetString(e))) // #nosec G203
}

func (g htmlStringGetter) JawsGetTag(rq *Request) any {
	return g.v
}

func makeHtmlGetter(v any) HtmlGetter {
	switch v := v.(type) {
	case HtmlGetter:
		return v
	case StringSetter:
		return htmlStringGetter{v}
	case template.HTML:
		return htmlGetter{v}
	case string:
		h := template.HTML(v) // #nosec G203
		return htmlGetter{h}
	case *atomic.Value:
		return atomicSetter{v}
	}
	panic(fmt.Errorf("expected jaws.HtmlGetter or string, not %T", v))
}
