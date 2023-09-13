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
}

func NewUiHtml(up Params) UiHtml {
	return UiHtml{
		Tags:    up.Tags(),
		EventFn: up.ef,
	}
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

func (ui *UiHtml) WriteHtmlInner(w io.Writer, e *Element, htmltag, htmltype string, htmlinner template.HTML, data []interface{}) {
	maybePanic(WriteHtmlInner(w, e.Jid(), htmltag, htmltype, htmlinner, e.Attrs()...))
}

func (ui *UiHtml) WriteHtmlSelect(w io.Writer, e *Element, nba *NamedBoolArray, data ...interface{}) {
	maybePanic(WriteHtmlSelect(w, e.Jid(), nba, e.Attrs()...))
}

func (ui *UiHtml) WriteHtmlInput(w io.Writer, e *Element, htmltype, htmlval string, data ...interface{}) {
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, htmlval, e.Attrs()...))
}

func (ui *UiHtml) JawsTags(rq *Request, inTags []interface{}) []interface{} {
	return append(inTags, ui.Tags...)
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer) {
	panic(fmt.Errorf("jaws: UiHtml.JawsRender(%v, %v) called", e, w))
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
