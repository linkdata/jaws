package jaws

import (
	"fmt"
	"html/template"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type UiHtml struct {
	ClickHandler ClickHandler
	EventHandler EventHandler
	EventFn      EventFn // legacy
	Tag          interface{}
}

func writeUiDebug(e *Element, w io.Writer) {
	if deadlock.Debug {
		var sb strings.Builder
		_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", e.jid, e.ui)
		for i, tag := range e.Request.TagsOf(e) {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(TagString(tag))
		}
		sb.WriteByte(']')
		_, _ = w.Write([]byte(strings.ReplaceAll(sb.String(), "-->", "==>") + " -->"))
	}
}

func (ui *UiHtml) parseGetter(e *Element, getter interface{}) {
	if getter != nil {
		if tagger, ok := getter.(TagGetter); ok {
			ui.Tag = tagger.JawsGetTag(e.Request)
		} else {
			ui.Tag = getter
		}
		e.Tag(ui.Tag)
		if ch, ok := getter.(ClickHandler); ok {
			ui.ClickHandler = ch
		}
		if eh, ok := getter.(EventHandler); ok {
			ui.EventHandler = eh
		}
	}
}

func (ui *UiHtml) parseParams(elem *Element, params []interface{}) (attrs []string) {
	for i := range params {
		switch data := params[i].(type) {
		case template.HTML:
			attrs = append(attrs, string(data))
		case []template.HTML:
			for _, s := range data {
				attrs = append(attrs, string(s))
			}
		case string:
			attrs = append(attrs, data)
		case []string:
			attrs = append(attrs, data...)
		case EventFn:
			ui.EventFn = data
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
		default:
			if h, ok := data.(ClickHandler); ok {
				ui.ClickHandler = h
			}
			if h, ok := data.(EventHandler); ok {
				ui.EventHandler = h
			}
			elem.Tag(data)
		}
	}
	return
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer, params []interface{}) {
	panic(fmt.Errorf("jaws: UiHtml.JawsRender(%v) called", e))
}

func (ui *UiHtml) JawsUpdate(u *Element) {
	switch v := ui.Tag.(type) {
	case *NamedBoolArray:
		u.SetValue(v.Get())
	case StringGetter:
		u.SetValue(v.JawsGetString(u))
	case FloatGetter:
		u.SetValue(string(fmt.Append(nil, v.JawsGetFloat(u))))
	case BoolGetter:
		if v.JawsGetBool(u) {
			u.SetAttr("checked", "")
		} else {
			u.RemoveAttr("checked")
		}
	case TimeGetter:
		u.SetValue(v.JawsGetTime(u).Format(ISO8601))
	case HtmlGetter:
		u.SetInner(v.JawsGetHtml(u))
	case UI:
		v.JawsUpdate(u)
	default:
		panic(fmt.Errorf("jaws: UiHtml.JawsUpdate(%v): unhandled type: %T", u, v))
	}
}

func (ui *UiHtml) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil { // LEGACY
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Click && ui.ClickHandler != nil {
		return ui.ClickHandler.JawsClick(e, val)
	}
	if ui.EventHandler != nil {
		return ui.EventHandler.JawsEvent(e, wht, val)
	}
	if wht == what.Input {
		switch data := ui.Tag.(type) {
		case *NamedBoolArray:
			data.Set(val, true)
		case StringSetter:
			err = data.JawsSetString(e, val)
		case FloatSetter:
			var v float64
			if val != "" {
				if v, err = strconv.ParseFloat(val, 64); err != nil {
					return
				}
			}
			err = data.JawsSetFloat(e, v)
		case BoolSetter:
			var v bool
			if val != "" {
				if v, err = strconv.ParseBool(val); err != nil {
					return
				}
			}
			err = data.JawsSetBool(e, v)
		case TimeSetter:
			var v time.Time
			if val != "" {
				if v, err = time.Parse(ISO8601, val); err != nil {
					return
				}
			}
			err = data.JawsSetTime(e, v)
		default:
			if deadlock.Debug {
				_ = e.Jaws.Log(fmt.Errorf("jaws: UiHtml.JawsEvent(%v, %s, %q): unhandled type: %T", e, wht, val, data))
			}
		}
		e.Jaws.Dirty(ui.Tag)
	}
	return
}
