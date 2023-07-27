package jaws

import (
	"html/template"
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

func (rq *Request) Text(tagstring, val string, fn InputTextFn, attrs ...interface{}) template.HTML {
	ui := &UiInputText{
		UiHtml:      UiHtml{Tags: StringTags(tagstring)},
		HtmlType:    "text",
		Value:       val,
		InputTextFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Password(tagstring string, fn InputTextFn, attrs ...interface{}) template.HTML {
	ui := &UiInputText{
		UiHtml:      UiHtml{Tags: StringTags(tagstring)},
		HtmlType:    "password",
		InputTextFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Select(nba *NamedBoolArray, fn InputTextFn, attrs ...interface{}) template.HTML {
	ui := &UiInputSelect{
		UiHtml:         UiHtml{Tags: StringTags(nba.Tagstring)},
		NamedBoolArray: nba,
		InputTextFn:    fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Number(tagstring string, val float64, fn InputFloatFn, attrs ...interface{}) template.HTML {
	ui := &UiInputFloat{
		UiHtml:       UiHtml{Tags: StringTags(tagstring)},
		HtmlType:     "number",
		Value:        val,
		InputFloatFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Range(tagstring string, val float64, fn InputFloatFn, attrs ...interface{}) template.HTML {
	ui := &UiInputFloat{
		UiHtml:       UiHtml{Tags: StringTags(tagstring)},
		HtmlType:     "range",
		Value:        val,
		InputFloatFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Checkbox(tagstring string, val bool, fn InputBoolFn, attrs ...interface{}) template.HTML {
	ui := &UiInputBool{
		UiHtml:      UiHtml{Tags: StringTags(tagstring)},
		HtmlType:    "checkbox",
		Value:       val,
		InputBoolFn: fn,
	}
	return rq.UI(ui, attrs...)
}

func (rq *Request) Radio(tagstring string, val bool, fn InputBoolFn, attrs ...interface{}) template.HTML {
	ui := &UiInputBool{
		UiHtml:      UiHtml{Tags: StringTags(tagstring)},
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
		UiHtml:      UiHtml{Tags: StringTags(tagstring)},
		Value:       val,
		InputDateFn: fn,
	}
	return rq.UI(ui, attrs...)
}
