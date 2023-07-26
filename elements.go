package jaws

import (
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/linkdata/jaws/what"
)

type ClickFn func(rq *Request, jid string) error
type InputTextFn func(rq *Request, jid string, val string) error
type InputFloatFn func(rq *Request, jid string, val float64) error
type InputBoolFn func(rq *Request, jid string, val bool) error
type InputDateFn func(rq *Request, jid string, val time.Time) error

const ISO8601 = "2006-01-02"

// OnInput registers a jid and a function to be called when it's input event fires.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnInput(jid string, fn InputTextFn) error {
	rq.maybeInputText(jid, fn)
	return nil
}

// OnClick registers a jid and a function to be called when it's click event fires.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnClick(jid string, fn ClickFn) error {
	rq.maybeClick(jid, fn)
	return nil
}

// OnTrigger registers a jid and a function to be called when Trigger is called for it.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnTrigger(jid string, fn ClickFn) error {
	rq.maybeEvent(what.Trigger, jid, fn)
	return nil
}

func (rq *Request) Div(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: tagstring},
		HtmlTag: "div",
		Text:    inner,
		EventFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Span(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: tagstring},
		HtmlTag: "span",
		Text:    inner,
		EventFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Li(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: tagstring},
		HtmlTag: "li",
		Text:    inner,
		EventFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Td(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: tagstring},
		HtmlTag: "td",
		Text:    inner,
		EventFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) A(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: tagstring},
		HtmlTag: "a",
		Text:    inner,
		EventFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Button(tagstring, txt string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:   UiBase{Tags: tagstring},
		HtmlTag:  "button",
		HtmlType: "button",
		Text:     txt,
		EventFn:  fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Img(tagstring, src string, fn ClickFn, attrs ...interface{}) template.HTML {
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	attrs = append(attrs, "src="+src)
	ui := &UiClickable{
		UiBase:  UiBase{Tags: tagstring},
		HtmlTag: "img",
		EventFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Text(jid, val string, fn InputTextFn, attrs ...string) template.HTML {
	return HtmlInput(rq.maybeInputText(jid, fn), "text", val, attrs...)
}

func (rq *Request) Password(jid string, fn InputTextFn, attrs ...string) template.HTML {
	return HtmlInput(rq.maybeInputText(jid, fn), "password", "", attrs...)
}

func (rq *Request) Number(jid string, val float64, fn InputFloatFn, attrs ...string) template.HTML {
	return HtmlInput(rq.maybeInputFloat(jid, fn), "number", strconv.FormatFloat(val, 'f', -1, 64), attrs...)
}

func (rq *Request) Range(jid string, val float64, fn InputFloatFn, attrs ...string) template.HTML {
	return HtmlInput(rq.maybeInputFloat(jid, fn), "range", strconv.FormatFloat(val, 'f', -1, 64), attrs...)
}

func (rq *Request) Checkbox(jid string, val bool, fn InputBoolFn, attrs ...string) template.HTML {
	if val {
		attrs = append(attrs, "checked")
	}
	return HtmlInput(rq.maybeInputBool(jid, fn), "checkbox", "", attrs...)
}

func (rq *Request) Date(jid string, val time.Time, fn InputDateFn, attrs ...string) template.HTML {
	if val.IsZero() {
		val = time.Now()
	}
	return HtmlInput(rq.maybeInputDate(jid, fn), "date", val.Format(ISO8601), attrs...)
}

func (rq *Request) Radio(jid string, val bool, fn InputBoolFn, attrs ...string) template.HTML {
	if val {
		attrs = append(attrs, "checked")
	}
	return HtmlInput(rq.maybeInputBool(jid, fn), "radio", "", attrs...)
}

func (rq *Request) Select(nba *NamedBoolArray, fn InputTextFn, attrs ...string) template.HTML {
	return HtmlSelect(rq.maybeInputText(nba.Jid, fn), nba, attrs...)
}
