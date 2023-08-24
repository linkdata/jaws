package jaws

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/linkdata/deadlock"
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
	return
}

func ProcessData(dataslice []interface{}) (attrs []string, eventFn EventFn) {
	for _, dataitem := range dataslice {
		switch data := dataitem.(type) {
		case string:
			attrs = append(attrs, data)
		case EventFn:
			if data != nil {
				eventFn = data
			}
		case func(*Request, string) error: // ClickFn
			if data != nil {
				eventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Click {
						err = data(rq, jid)
					}
					return
				}
			}
		case func(*Request, string, string) error: // InputTextFn
			if data != nil {
				eventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
					if wht == what.Input {
						err = data(rq, jid, val)
					}
					return
				}
			}
		case func(*Request, string, bool) error: // InputBoolFn
			if data != nil {
				eventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
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
				eventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
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
				eventFn = func(rq *Request, wht what.What, jid, val string) (err error) {
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

func htmlValueString(val interface{}) (s string) {
	switch v := val.(type) {
	case nil:
		s = "null"
	case string:
		s = v
	case int:
		s = strconv.Itoa(v)
	case bool:
		if v {
			s = "true"
		} else {
			s = "false"
		}
	case time.Time:
		s = v.Format(ISO8601)
	case float64:
		s = strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		s = strconv.FormatFloat(float64(v), 'f', -1, 32)
	default:
		panic(fmt.Errorf("jaws: don't know how to convert %T into HTML value", val))
	}
	return
}

func (ui *UiHtml) ProcessData(dataslice []interface{}) []string {
	attrs, eventFn := ProcessData(dataslice)
	if eventFn != nil {
		ui.EventFn = eventFn
	}
	return attrs
}

func (ui *UiHtml) writeDebug(w io.Writer, jid string) {
	if deadlock.Debug {
		w.Write([]byte(strings.ReplaceAll(fmt.Sprintf("<!-- jid=%s tags: %v", jid, ui.Tags), "-->", "") + " -->"))
	}
}

func (ui *UiHtml) WriteHtmlInner(w io.Writer, htmltag, htmltype, htmlinner, jid string, data []interface{}) error {
	if deadlock.Debug {
		ui.writeDebug(w, jid)
	}
	return WriteHtmlInner(w, jid, htmltag, htmltype, htmlinner, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlSelect(w io.Writer, nba *NamedBoolArray, jid string, data ...interface{}) error {
	if deadlock.Debug {
		ui.writeDebug(w, jid)
	}
	return WriteHtmlSelect(w, jid, nba, ui.ProcessData(data)...)
}

func (ui *UiHtml) WriteHtmlInput(w io.Writer, htmltype, htmlval, jid string, data ...interface{}) error {
	if deadlock.Debug {
		ui.writeDebug(w, jid)
	}
	return WriteHtmlInput(w, jid, htmltype, htmlval, ui.ProcessData(data)...)
}

func (ui *UiHtml) JawsCreate(rq *Request, data []interface{}) (elem *Element, err error) {
	return rq.NewElement(ui.Tags, ui, data), nil
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer) (err error) {
	panic(fmt.Sprintf("jaws: UiHtml.JawsRender(%v, %v) called", e, w))
}

func (ui *UiHtml) JawsUpdate(e *Element) (err error) {
	panic(fmt.Sprintf("jaws: UiHtml.JawsUpdate(%v) called", e))
}

func (ui *UiHtml) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		err = ui.EventFn(e.Request(), wht, e.Jid().String(), val)
	}
	return
}
