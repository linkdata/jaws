package jaws

import (
	"html/template"
	"sync/atomic"
	"time"
)

type atomicGetter struct{ v *atomic.Value }

func (g atomicGetter) JawsGetBool(e *Element) bool {
	return g.v.Load().(bool)
}

func (g atomicGetter) JawsSetBool(e *Element, v bool) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetFloat(e *Element) float64 {
	return g.v.Load().(float64)
}

func (g atomicGetter) JawsSetFloat(e *Element, v float64) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetString(e *Element) string {
	return g.v.Load().(string)
}

func (g atomicGetter) JawsSetString(e *Element, v string) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetTime(e *Element) time.Time {
	return g.v.Load().(time.Time)
}

func (g atomicGetter) JawsSetTime(e *Element, v time.Time) (err error) {
	g.v.Store(v)
	return
}

func (g atomicGetter) JawsGetHtml(e *Element) template.HTML {
	switch v := g.v.Load().(type) {
	case template.HTML:
		return v
	case string:
		return template.HTML(v)
	}
	panic("atomicGetter.JawsGetHtml: unsupported type")
}

func (g atomicGetter) JawsGetTag(e *Element) interface{} {
	return g.v
}
