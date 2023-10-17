package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync/atomic"
	"time"
)

type atomicGetter struct{ v *atomic.Value }

func (g atomicGetter) JawsGetBool(e *Element) (v bool) {
	if x := g.v.Load(); x != nil {
		v = x.(bool)
	}
	return
}

func (g atomicGetter) JawsSetBool(e *Element, v bool) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetFloat(e *Element) (v float64) {
	if x := g.v.Load(); x != nil {
		v = x.(float64)
	}
	return
}

func (g atomicGetter) JawsSetFloat(e *Element, v float64) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetString(e *Element) (v string) {
	if x := g.v.Load(); x != nil {
		v = x.(string)
	}
	return
}

func (g atomicGetter) JawsSetString(e *Element, v string) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetTime(e *Element) (v time.Time) {
	if x := g.v.Load(); x != nil {
		v = x.(time.Time)
	}
	return
}

func (g atomicGetter) JawsSetTime(e *Element, v time.Time) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetHtml(e *Element) template.HTML {
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

func (g atomicGetter) JawsGetTag(rq *Request) any {
	return g.v
}
