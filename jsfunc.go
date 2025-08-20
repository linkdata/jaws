package jaws

import (
	"io"
	"strconv"
)

type IsJsFunc interface {
	UI
	IsJsFunc()
}

type JsFunc string

var _ IsJsFunc = JsFunc("")

func (ui JsFunc) IsJsFunc() {}

func (ui JsFunc) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	e.Tag(ui)
	attrs := e.ApplyParams(params)
	var b []byte
	b = append(b, "\n<div id="...)
	b = e.Jid().AppendQuote(b)
	b = append(b, " data-jawsname="...)
	b = strconv.AppendQuote(b, string(ui))
	b = appendAttrs(b, attrs)
	b = append(b, " hidden></div>"...)
	_, err = w.Write(b)
	return
}

func (ui JsFunc) JawsUpdate(e *Element) {} // no-op, use JawsCall to invoke

func NewJsFunc(name string) JsFunc {
	return JsFunc(name)
}

func (rq RequestWriter) JsFunc(arg any, params ...any) (err error) {
	var jsfunc JsFunc
	switch arg := arg.(type) {
	case string:
		jsfunc = NewJsFunc(arg)
	case JsFunc:
		jsfunc = arg
	}
	return rq.UI(jsfunc, params...)
}
