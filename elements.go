package jaws

import (
	"html/template"
	"strconv"
	"strings"
	"time"
)

type ClickFn func(rq *Request) error
type InputTextFn func(rq *Request, val string) error
type InputFloatFn func(rq *Request, val float64) error
type InputBoolFn func(rq *Request, val bool) error
type InputDateFn func(rq *Request, val time.Time) error

const ISO8601 = "2006-01-02"

func (rq *Request) maybeClick(id string, fn ClickFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "click" {
				err = fn(rq)
			}
			return
		}
	}
	return rq.RegisterEventFn(id, wf)
}

func (rq *Request) maybeInputText(id string, fn InputTextFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				err = fn(rq, val)
			}
			return
		}
	}
	return rq.RegisterEventFn(id, wf)
}

func (rq *Request) maybeInputFloat(id string, fn InputFloatFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				var v float64
				if v, err = strconv.ParseFloat(val, 64); err == nil {
					err = fn(rq, v)
				}
			}
			return
		}
	}
	return rq.RegisterEventFn(id, wf)
}

func (rq *Request) maybeInputBool(id string, fn InputBoolFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				var v bool
				if v, err = strconv.ParseBool(val); err == nil {
					err = fn(rq, v)
				}
			}
			return
		}
	}
	return rq.RegisterEventFn(id, wf)
}

func (rq *Request) maybeInputDate(id string, fn InputDateFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				var v time.Time
				if v, err = time.Parse(ISO8601, val); err == nil {
					err = fn(rq, v)
				}
			}
			return
		}
	}
	return rq.RegisterEventFn(id, wf)
}

// OnInput registers a HTML id and a function to be called when it's input event fires.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnInput(id string, fn InputTextFn) error {
	rq.maybeInputText(id, fn)
	return nil
}

// OnClick registers a HTML id and a function to be called when it's click event fires.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnClick(id string, fn ClickFn) error {
	rq.maybeClick(id, fn)
	return nil
}

func (rq *Request) Div(id, inner string, fn ClickFn, attrs string) template.HTML {
	return HtmlInner(rq.maybeClick(id, fn), "div", "", inner, attrs)
}

func (rq *Request) Span(id, inner string, fn ClickFn, attrs string) template.HTML {
	return HtmlInner(rq.maybeClick(id, fn), "span", "", inner, attrs)
}

func (rq *Request) Li(id, inner string, fn ClickFn, attrs string) template.HTML {
	return HtmlInner(rq.maybeClick(id, fn), "li", "", inner, attrs)
}

func (rq *Request) Td(id, inner string, fn ClickFn, attrs string) template.HTML {
	return HtmlInner(rq.maybeClick(id, fn), "td", "", inner, attrs)
}

func (rq *Request) A(id, inner string, fn ClickFn, attrs string) template.HTML {
	return HtmlInner(rq.maybeClick(id, fn), "a", "", inner, attrs)
}

func (rq *Request) Button(id, txt string, fn ClickFn, attrs string) template.HTML {
	return HtmlInner(rq.maybeClick(id, fn), "button", "button", txt, attrs)
}

func (rq *Request) Text(id, val string, fn InputTextFn, attrs string) template.HTML {
	return HtmlInput(rq.maybeInputText(id, fn), "text", val, attrs)
}

func (rq *Request) Password(id string, fn InputTextFn, attrs string) template.HTML {
	return HtmlInput(rq.maybeInputText(id, fn), "password", "", attrs)
}

func (rq *Request) Number(id string, val float64, fn InputFloatFn, attrs string) template.HTML {
	return HtmlInput(rq.maybeInputFloat(id, fn), "number", strconv.FormatFloat(val, 'f', -1, 64), attrs)
}

func (rq *Request) Range(id string, val float64, fn InputFloatFn, attrs string) template.HTML {
	return HtmlInput(rq.maybeInputFloat(id, fn), "range", strconv.FormatFloat(val, 'f', -1, 64), attrs)
}

func (rq *Request) Checkbox(id string, val bool, fn InputBoolFn, attrs string) template.HTML {
	if val {
		attrs += " checked"
	}
	return HtmlInput(rq.maybeInputBool(id, fn), "checkbox", "", attrs)
}

func (rq *Request) Date(id string, val time.Time, fn InputDateFn, attrs string) template.HTML {
	if val.IsZero() {
		val = time.Now()
	}
	return HtmlInput(rq.maybeInputDate(id, fn), "date", val.Format(ISO8601), attrs)
}

func radioGroup(id string) string {
	if slash := strings.IndexByte(id, '/'); slash != -1 {
		return id[slash+1:]
	}
	panic("radio button ID's must be in the form 'buttonid/groupid'")
}

func (rq *Request) Radio(id string, val bool, fn InputBoolFn, attrs string) template.HTML {
	attrs += " name=\"" + radioGroup(id) + "\""
	if val {
		attrs += " checked"
	}
	return HtmlInput(rq.maybeInputBool(id, fn), "radio", "", attrs)
}

func (rq *Request) Select(id string, val *NamedBoolArray, fn InputTextFn, attrs string) template.HTML {
	return HtmlSelect(rq.maybeInputText(id, fn), val, attrs)
}

func (rq *Request) Ui(elem Ui, attrs ...string) template.HTML {
	return elem.UiHTML(rq.RegisterEventFn(elem.UiID(), elem.UiEvent), attrs...)
}
