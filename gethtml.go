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

func (g htmlGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

type htmlStringGetter struct{ v StringGetter }

func (g htmlStringGetter) JawsGetHtml(e *Element) template.HTML {
	return template.HTML(html.EscapeString(g.v.JawsGetString(e)))
}

func (g htmlStringGetter) JawsGetTag(rq *Request) interface{} {
	return g.v
}

func makeHtmlGetter(v interface{}) HtmlGetter {
	switch v := v.(type) {
	case HtmlGetter:
		return v
	case StringGetter:
		return htmlStringGetter{v}
	case template.HTML:
		return htmlGetter{v}
	case string:
		return htmlGetter{template.HTML(v)}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic(fmt.Sprintf("expected jaws.HtmlGetter or string, not %T", v))
}
