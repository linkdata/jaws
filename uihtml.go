package jaws

import (
	"io"
	"strings"
	"time"

	"github.com/linkdata/jaws/what"
)

type ClickFn func(rq *Request, jid string) error
type InputTextFn func(rq *Request, jid string, val string) error
type InputFloatFn func(rq *Request, jid string, val float64) error
type InputBoolFn func(rq *Request, jid string, val bool) error
type InputDateFn func(rq *Request, jid string, val time.Time) error

type UiHtml struct {
	Tags    []interface{}
	EventFn EventFn
}

func StringTags(text string) (tags []interface{}) {
	for _, s := range strings.Split(text, " ") {
		if s != "" {
			tags = append(tags, s)
		}
	}
	return
}

func (ui *UiHtml) ProcessData(data []interface{}) (attrs []string) {
	for _, v := range data {
		switch v := v.(type) {
		case string:
			attrs = append(attrs, v)
		case EventFn:
			if v != nil {
				ui.EventFn = v
			}
		case ClickFn:
			if v != nil {
				ui.EventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Click {
						err = v(rq, jid)
					}
					return
				}
			}
		case InputBoolFn:
		case InputTextFn:
		case InputFloatFn:
		case InputDateFn:
		}
	}
	return
}

func (ui *UiHtml) WriteHtmlInner(rq *Request, w io.Writer, htmltag, htmltype, htmlinner, jid string, data ...interface{}) error {
	return WriteHtmlInner(w, jid, htmltag, htmltype, htmlinner, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlInput(rq *Request, w io.Writer, htmltype, htmlval, jid string, data ...interface{}) error {
	return WriteHtmlInput(w, jid, htmltype, htmlval, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlSelect(rq *Request, w io.Writer, nba *NamedBoolArray, jid string, data ...interface{}) error {
	return WriteHtmlSelect(w, jid, nba, ui.ProcessData(data)...)
}

func (uib *UiHtml) JawsTags(rq *Request) (tags []interface{}) {
	return uib.Tags
}

func (uib *UiHtml) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) (err error) {
	return
}

func (uib *UiHtml) JawsEvent(rq *Request, wht what.What, jid string, val string) (err error) {
	if uib.EventFn != nil {
		err = uib.EventFn(rq, wht, jid, val)
	}
	return
}
