package jaws

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiHtml struct {
	Tags    []interface{}
	EventFn EventFn
}

func stringTags(tags []interface{}, text string) []interface{} {
	for _, s := range strings.Split(text, " ") {
		if s != "" {
			tags = append(tags, s)
		}
	}
	return tags
}

func ProcessTags(tagsitem interface{}) (tags []interface{}) {
	switch data := tagsitem.(type) {
	case string:
		tags = stringTags(tags, data)
	case []string:
		for _, s := range data {
			tags = stringTags(tags, s)
		}
	case []interface{}:
		tags = append(tags, data...)
	default:
		tags = append(tags, data)
	}
	if len(tags) == 0 {
		tags = append(tags, MakeID())
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

func (ui *UiHtml) WriteHtmlInner(w io.Writer, htmltag, htmltype, htmlinner, jid string, data ...interface{}) error {
	return WriteHtmlInner(w, jid, htmltag, htmltype, htmlinner, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlSelect(w io.Writer, nba *NamedBoolArray, jid string, data ...interface{}) error {
	return WriteHtmlSelect(w, jid, nba, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlInput(w io.Writer, htmltype, htmlval, jid string, data ...interface{}) error {
	return WriteHtmlInput(w, jid, htmltype, htmlval, ui.ProcessData(data)...)
}

func (ui *UiHtml) JawsTags(rq *Request) (tags []interface{}) {
	return ui.Tags
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer) (err error) {
	panic(fmt.Sprintf("jaws: UiHtml.JawsRender(%v, %v) called", e, w))
}

func (ui *UiHtml) JawsUpdate(e *Element) (err error) {
	panic(fmt.Sprintf("jaws: UiHtml.JawsUpdate(%v) called", e))
}

func (ui *UiHtml) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		err = ui.EventFn(e.Request(), wht, e.Jid(), val)
	}
	return
}
