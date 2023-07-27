package jaws

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/linkdata/jaws/what"
)

type InputTextFn = func(rq *Request, jid string, val string) error

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

func (ui *UiHtml) ProcessData(dataslice []interface{}) (attrs []string) {
	for _, dataitem := range dataslice {
		switch data := dataitem.(type) {
		case string:
			attrs = append(attrs, data)
		case EventFn:
			if data != nil {
				ui.EventFn = data
			}
		case func(*Request, string) error: // ClickFn
			if data != nil {
				ui.EventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Click {
						err = data(rq, jid)
					}
					return
				}
			}
		case func(*Request, string, string) error: // InputTextFn
			if data != nil {
				ui.EventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						err = data(rq, jid, val)
					}
					return
				}
			}
		case func(*Request, string, bool) error: // InputBoolFn
			if data != nil {
				ui.EventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v bool
						if val != "" {
							if v, err = strconv.ParseBool(val); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		case func(*Request, string, float64) error: // InputFloatFn
			if data != nil {
				ui.EventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v float64
						if val != "" {
							if v, err = strconv.ParseFloat(val, 64); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		case func(*Request, string, time.Time) error: // InputDateFn
			if data != nil {
				ui.EventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						var v time.Time
						if val != "" {
							if v, err = time.Parse(ISO8601, val); err != nil {
								return
							}
						}
						err = data(rq, jid, v)
					}
					return
				}
			}
		}
	}
	return
}

func (ui *UiHtml) WriteHtmlInner(rq *Request, w io.Writer, htmltag, htmltype, htmlinner, jid string, data ...interface{}) error {
	return WriteHtmlInner(w, jid, htmltag, htmltype, htmlinner, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlSelect(rq *Request, w io.Writer, nba *NamedBoolArray, jid string, data ...interface{}) error {
	return WriteHtmlSelect(w, jid, nba, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlInput(rq *Request, w io.Writer, htmltype, htmlval, jid string, data ...interface{}) error {
	return WriteHtmlInput(w, jid, htmltype, htmlval, ui.ProcessData(data)...)
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
