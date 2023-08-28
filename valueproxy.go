package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync/atomic"
)

type ValueReader interface {
	JawsGet(e *Element) (val interface{})
}

type ValueProxy interface {
	ValueReader
	JawsSet(e *Element, val interface{}) (changed bool)
}

type DummyReader struct{ Value interface{} }

func (vp DummyReader) JawsGet(e *Element) interface{} {
	return vp.Value
}

type AtomicReader struct{ *atomic.Value }

func (vp AtomicReader) JawsGet(e *Element) interface{} {
	return vp.Load()
}

type AtomicProxy struct{ *atomic.Value }

func (vp AtomicProxy) JawsGet(e *Element) interface{} {
	return vp.Load()
}

func (vp AtomicProxy) JawsSet(e *Element, val interface{}) (changed bool) {
	changed = vp.Swap(val) != val
	return
}

func MakeValueProxy(value interface{}) ValueProxy {
	switch v := value.(type) {
	case ValueProxy:
		return v
	case *atomic.Value:
		return AtomicProxy{Value: v}
	case atomic.Value:
		panic("jaws: MakeValueProxy: must pass atomic.Value by reference")
	}
	panic("jaws: MakeValueProxy: expected ValueProxy or *atomic.Value")
}

func MakeValueReader(value interface{}) ValueReader {
	switch v := value.(type) {
	case ValueReader:
		return v
	case *atomic.Value:
		return AtomicReader{Value: v}
	case atomic.Value:
		panic("jaws: MakeValueReader: must pass atomic.Value by reference")
	}
	panic("jaws: MakeValueReader: expected ValueReader or *atomic.Value")
}

func anyToHtml(val interface{}) template.HTML {
	var s string
	switch v := val.(type) {
	case template.HTML:
		return v
	case *atomic.Value:
		return anyToHtml(v.Load())
	case string:
		s = v
	case fmt.Stringer:
		s = v.String()
	default:
		panic(fmt.Sprintf("jaws: unable to make HTML from %T", v))
	}
	return template.HTML(html.EscapeString(s))
}
