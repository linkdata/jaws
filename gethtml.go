package jaws

import (
	"html/template"
	"sync/atomic"
)

type HtmlGetter interface {
	JawsGetHtml(rq *Element) template.HTML
}

type htmlGetter struct{ v template.HTML }

func (g htmlGetter) JawsGetHtml(rq *Element) template.HTML {
	return g.v
}

func (g htmlGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

func makeHtmlGetter(v interface{}) HtmlGetter {
	switch v := v.(type) {
	case HtmlGetter:
		return v
	case template.HTML:
		return htmlGetter{v}
	case string:
		return htmlGetter{template.HTML(v)}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic("makeHtmlGetter: invalid type")
}
