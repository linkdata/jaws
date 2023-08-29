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
		w.Write([]byte(strings.ReplaceAll(fmt.Sprintf("<!-- jid=%s %T tags: %v", e.jid, e.ui, e.tags), "-->", "") + " -->"))
	}
}

func (ui *UiHtml) WriteHtmlInner(w io.Writer, e *Element, htmltag, htmltype string, htmlinner template.HTML, data []interface{}) error {
	return WriteHtmlInner(w, e.Jid(), htmltag, htmltype, htmlinner, e.Attrs()...)
}

func (ui *UiHtml) WriteHtmlSelect(w io.Writer, e *Element, nba *NamedBoolArray, data ...interface{}) error {
	return WriteHtmlSelect(w, e.Jid(), nba, e.Attrs()...)
}

func (ui *UiHtml) WriteHtmlInput(w io.Writer, e *Element, htmltype, htmlval string, data ...interface{}) error {
	return WriteHtmlInput(w, e.Jid(), htmltype, htmlval, e.Attrs()...)
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
		err = ui.EventFn(e.Request(), wht, e.Jid().String(), val)
	} else if deadlock.Debug {
		log.Println("UiHtml.JawsEvent() ignored", e.Jid(), wht, val)
	}
	return
}
