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

// OnTrigger registers a jid and a function to be called when Trigger is called for it.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnTrigger(jid string, fn ClickFn) error {
	rq.maybeEvent(what.Trigger, jid, fn)
	return nil
}

func (rq *Request) Div(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: StringTags(tagstring)},
		HtmlTag: "div",
		Text:    inner,
		ClickFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Span(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: StringTags(tagstring)},
		HtmlTag: "span",
		Text:    inner,
		ClickFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Li(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: StringTags(tagstring)},
		HtmlTag: "li",
		Text:    inner,
		ClickFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Td(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: StringTags(tagstring)},
		HtmlTag: "td",
		Text:    inner,
		ClickFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) A(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:  UiBase{Tags: StringTags(tagstring)},
		HtmlTag: "a",
		Text:    inner,
		ClickFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Button(tagstring, txt string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiClickable{
		UiBase:   UiBase{Tags: StringTags(tagstring)},
		HtmlTag:  "button",
		HtmlType: "button",
		Text:     txt,
		ClickFn:  fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Img(tagstring, src string, fn ClickFn, attrs ...interface{}) template.HTML {
	if !strings.HasPrefix(src, "\"") {
		src = strconv.Quote(src)
	}
	attrs = append(attrs, "src="+src)
	ui := &UiClickable{
		UiBase:  UiBase{Tags: StringTags(tagstring)},
		HtmlTag: "img",
		ClickFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Text(tagstring, val string, fn InputTextFn, attrs ...interface{}) template.HTML {
	ui := &UiInputText{
		UiBase:      UiBase{Tags: StringTags(tagstring)},
		HtmlType:    "text",
		Value:       val,
		InputTextFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Password(tagstring string, fn InputTextFn, attrs ...interface{}) template.HTML {
	ui := &UiInputText{
		UiBase:      UiBase{Tags: StringTags(tagstring)},
		HtmlType:    "password",
		InputTextFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Select(nba *NamedBoolArray, fn InputTextFn, attrs ...interface{}) template.HTML {
	ui := &UiInputSelect{
		UiBase:         UiBase{Tags: StringTags(nba.Tagstring)},
		NamedBoolArray: nba,
		InputTextFn:    fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Number(tagstring string, val float64, fn InputFloatFn, attrs ...interface{}) template.HTML {
	ui := &UiInputFloat{
		UiBase:       UiBase{Tags: StringTags(tagstring)},
		HtmlType:     "number",
		Value:        val,
		InputFloatFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Range(tagstring string, val float64, fn InputFloatFn, attrs ...interface{}) template.HTML {
	ui := &UiInputFloat{
		UiBase:       UiBase{Tags: StringTags(tagstring)},
		HtmlType:     "range",
		Value:        val,
		InputFloatFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Checkbox(tagstring string, val bool, fn InputBoolFn, attrs ...interface{}) template.HTML {
	ui := &UiInputBool{
		UiBase:      UiBase{Tags: StringTags(tagstring)},
		HtmlType:    "checkbox",
		Value:       val,
		InputBoolFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Radio(tagstring string, val bool, fn InputBoolFn, attrs ...interface{}) template.HTML {
	ui := &UiInputBool{
		UiBase:      UiBase{Tags: StringTags(tagstring)},
		HtmlType:    "radio",
		Value:       val,
		InputBoolFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Date(tagstring string, val time.Time, fn InputDateFn, attrs ...interface{}) template.HTML {
	if val.IsZero() {
		val = time.Now()
	}
	ui := &UiInputDate{
		UiBase:      UiBase{Tags: StringTags(tagstring)},
		Value:       val,
		InputDateFn: fn,
	}
	return rq.UI(ui, attrs...)
}
