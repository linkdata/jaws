package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync/atomic"
	"time"
)

type atomicSetter struct{ v *atomic.Value }

func (g atomicSetter) JawsGetBool(e *Element) (v bool) {
	if x := g.v.Load(); x != nil {
		v = x.(bool)
	}
	return
}

func (g atomicSetter) JawsSetBool(e *Element, v bool) (err error) {
	g.v.Store(v)
	return
}

func (g atomicSetter) JawsGetFloat(e *Element) (v float64) {
	if x := g.v.Load(); x != nil {
		v = x.(float64)
	}
	return
}

func (g atomicSetter) JawsSetFloat(e *Element, v float64) (err error) {
	g.v.Store(v)
	return
}

func (g atomicSetter) JawsGetString(e *Element) (v string) {
	if x := g.v.Load(); x != nil {
		v = x.(string)
	}
	return
}

func (g atomicSetter) JawsSetString(e *Element, v string) (err error) {
	g.v.Store(v)
	return
}

func (g atomicSetter) JawsGetTime(e *Element) (v time.Time) {
	if x := g.v.Load(); x != nil {
		v = x.(time.Time)
	}
	return
}

func (g atomicSetter) JawsSetTime(e *Element, v time.Time) (err error) {
	g.v.Store(v)
	return
}

func (g atomicSetter) JawsGetHtml(e *Element) template.HTML {
	switch v := g.v.Load().(type) {
	case nil:
		return ""
	case template.HTML:
		return v
	default:
		h := template.HTML(html.EscapeString(fmt.Sprint(v))) // #nosec G203
		return h
	}
}

func (g atomicSetter) JawsGetTag(rq *Request) any {
	return g.v
}
