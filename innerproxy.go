package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync/atomic"
)

func innerProxyStringToHtml(val interface{}) template.HTML {
	switch v := val.(type) {
	case template.HTML:
		return v
	case string:
		return template.HTML(v)
	case fmt.Stringer:
		return template.HTML(html.EscapeString(v.String()))
	case *atomic.Value:
		return innerProxyStringToHtml(v.Load())
	}
	panic("jaws.InnerProxy: not a stringable object")
}

type InnerProxy interface {
	JawsInner(e *Element) template.HTML
}

type defaultInnerProxy struct{ v interface{} }

func (vp defaultInnerProxy) JawsInner(e *Element) template.HTML {
	return innerProxyStringToHtml(vp.v)
}

func MakeInnerProxy(value interface{}) (ip InnerProxy) {
	switch v := value.(type) {
	case InnerProxy:
		ip = v
	default:
		ip = defaultInnerProxy{v: v}
	}
	return
}
