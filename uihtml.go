package jaws

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type UiHtml struct {
	Tags    []interface{}
	EventFn EventFn
	Attrs   []string
}

func NewUiHtml() UiHtml {
	return UiHtml{}
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

func writeUiDebug(e *Element, w io.Writer) {
	if deadlock.Debug {
		var sb strings.Builder
		_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", e.jid, e.ui)
		for i, tag := range e.Tags() {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(TagString(tag))
		}
		sb.WriteByte(']')
		_, _ = w.Write([]byte(strings.ReplaceAll(sb.String(), "-->", "==>") + " -->"))
	}
}

func (ui *UiHtml) WriteHtmlInner(w io.Writer, e *Element, htmltag, htmltype string, htmlinner template.HTML, params ...interface{}) {
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInner(w, e.Jid(), htmltag, htmltype, htmlinner, ui.Attrs...))
}

func (ui *UiHtml) WriteHtmlSelect(w io.Writer, e *Element, nba *NamedBoolArray, params ...interface{}) {
	writeUiDebug(e, w)
	maybePanic(WriteHtmlSelect(w, e.Jid(), nba, ui.Attrs...))
}

func (ui *UiHtml) WriteHtmlInput(w io.Writer, e *Element, htmltype, htmlval string, params ...interface{}) {
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, htmlval, ui.Attrs...))
}

func (ui *UiHtml) JawsTags(rq *Request, inTags []interface{}) []interface{} {
	return append(inTags, ui.Tags...)
}

func (ui *UiHtml) ExtractParams(rq *Request, vp ValueProxy, params []interface{}) []interface{} {
	if tagger, ok := vp.(Tagger); ok {
		ui.Tags = tagger.JawsTags(rq, ui.Tags)
	}
	var remains int
	for i := range params {
		switch data := params[i].(type) {
		case Tag:
			ui.Tags = append(ui.Tags, data.Value)
		case template.HTML:
			ui.Attrs = append(ui.Attrs, string(data))
		case string:
			ui.Attrs = append(ui.Attrs, data)
		case EventFn:
			ui.EventFn = data
		case []string:
			ui.Attrs = append(ui.Attrs, data...)
		case []template.HTML:
			for _, s := range data {
				ui.Attrs = append(ui.Attrs, string(s))
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

		default:
			params[remains] = params[i]
			remains++
		}
	}
	return params[:remains]
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	panic(fmt.Errorf("jaws: UiHtml.JawsRender(%v) called", e))
}

func (ui *UiHtml) JawsUpdate(u Updater) {
	panic(fmt.Errorf("jaws: UiHtml.JawsUpdate(%v) called", u.Element))
}

func (ui *UiHtml) JawsEvent(e *Element, wht what.What, val string) error {
	if ui.EventFn != nil { // LEGACY
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	// see if one of our tags is a handler
	tags := append(e.Tags(), ui.Tags...)
	if wht == what.Click {
		for _, tag := range tags {
			if ch, ok := tag.(ClickHandler); ok {
				return ch.JawsClick(e, val)
			}
		}
	}
	for _, tag := range tags {
		if eh, ok := tag.(EventHandler); ok {
			return eh.JawsEvent(e, wht, val)
		}
	}
	if deadlock.Debug && wht != what.Click {
		log.Printf("jaws: unhandled JawsEvent(%v, %q, %q)\n", e, wht, val)
	}
	return nil
}
