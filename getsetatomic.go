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
	v, _ = g.v.Load().(bool)
	return
}

func (g atomicGetter) JawsSetBool(e *Element, v bool) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetFloat(e *Element) (v float64) {
	v, _ = g.v.Load().(float64)
	return
}

func (g atomicGetter) JawsSetFloat(e *Element, v float64) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetString(e *Element) (v string) {
	v, _ = g.v.Load().(string)
	return
}

func (g atomicGetter) JawsSetString(e *Element, v string) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetTime(e *Element) (v time.Time) {
	v, _ = g.v.Load().(time.Time)
	return
}

func (g atomicGetter) JawsSetTime(e *Request, v time.Time) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetHtml(e *Element) template.HTML {
	switch v := g.v.Load().(type) {
	case template.HTML:
		return v
	default:
		return template.HTML(html.EscapeString(string(fmt.Append(nil, v))))
	}
}

func (g atomicGetter) JawsGetTag(rq *Request) any {
	return g.v
}
