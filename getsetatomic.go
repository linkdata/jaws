package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync/atomic"
	"time"
)

type atomicGetter struct{ v *atomic.Value }

func (g atomicGetter) JawsGetBool(rq *Request) bool {
	return g.v.Load().(bool)
}

func (g atomicGetter) JawsSetBool(rq *Request, v bool) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetFloat(rq *Request) float64 {
	return g.v.Load().(float64)
}

func (g atomicGetter) JawsSetFloat(rq *Request, v float64) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetString(rq *Request) string {
	return g.v.Load().(string)
}

func (g atomicGetter) JawsSetString(rq *Request, v string) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetTime(rq *Request) time.Time {
	return g.v.Load().(time.Time)
}

func (g atomicGetter) JawsSetTime(rq *Request, v time.Time) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetHtml(rq *Request) template.HTML {
	switch v := g.v.Load().(type) {
	case template.HTML:
		return v
	case string:
		return template.HTML(v)
	default:
		return template.HTML(html.EscapeString(string(fmt.Append(nil, v))))
	}
}

func (g atomicGetter) JawsGetTag(rq *Request) interface{} {
	return g.v
}
