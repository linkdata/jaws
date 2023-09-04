package jaws

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"sync/atomic"
)

type ValueProxy interface {
	JawsGet(e *Element) (val interface{})
	JawsSet(e *Element, val interface{}) (changed bool)
}

type atomicProxy struct{ *atomic.Value }

func (vp atomicProxy) JawsGet(e *Element) interface{}           { return vp.Load() }
func (vp atomicProxy) JawsSet(e *Element, val interface{}) bool { return vp.Swap(val) != val }

type readonlyProxy struct{ Value interface{} }

func (vp readonlyProxy) JawsGet(e *Element) interface{} { return vp.Value }
func (vp readonlyProxy) JawsSet(e *Element, val interface{}) bool {
	panic(fmt.Sprintf("jaws: Element %v: ValueProxy for %T is read-only", e, val))
}

func anyToHtml(val interface{}) template.HTML {
	var s string
	switch v := val.(type) {
	case string:
		s = v
	case template.HTML:
		return v
	case *atomic.Value:
		return anyToHtml(v.Load())
	case fmt.Stringer:
		s = v.String()
	case float64:
		s = strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		s = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		s = strconv.Itoa(v)
	default:
		panic(fmt.Sprintf("jaws: don't know how to render %T as template.HTML", v))
	}
	return template.HTML(html.EscapeString(s))
}
