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
type InputIntFn func(rq *Request, val int) error
type InputBoolFn func(rq *Request, val bool) error
type InputDateFn func(rq *Request, val time.Time) error

const ISO8601 = "2006-01-02"

func (rq *Request) inputHTML(id, typ, val, attrs string) template.HTML {
	need := 11 + len(id) + 8 + len(typ) + 9 + len(val) + 1 + 1 + len(attrs) + 1
	b := make([]byte, 0, need)
	b = append(b, `<input id="`...)
	b = append(b, id...)
	b = append(b, `" type="`...)
	b = append(b, typ...)
	if val != "" {
		b = append(b, `" value="`...)
		b = append(b, val...)
	}
	b = append(b, '"')
	if attrs != "" {
		b = append(b, ' ')
		b = append(b, attrs...)
	}
	b = append(b, '>')
	return template.HTML(b) // #nosec G203
}

func (rq *Request) innerHTML(id, tag, typ, inner, attrs string) template.HTML {
	need := 1 + len(tag)*2 + 5 + len(id) + 8 + len(typ) + 1 + 1 + len(attrs) + 1 + len(inner) + 2 + 1
	b := make([]byte, 0, need)
	b = append(b, '<')
	b = append(b, tag...)
	b = append(b, ` id="`...)
	b = append(b, id...)
	if typ != "" {
		b = append(b, `" type="`...)
		b = append(b, typ...)
	}
	b = append(b, '"')
	if attrs != "" {
		b = append(b, ' ')
		b = append(b, attrs...)
	}
	b = append(b, '>')
	b = append(b, inner...)
	b = append(b, "</"...)
	b = append(b, tag...)
	b = append(b, '>')
	return template.HTML(b) // #nosec G203
}

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

func (rq *Request) maybeInputInt(id string, fn InputIntFn) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, id, evt, val string) (err error) {
			if evt == "input" {
				var v int
				if v, err = strconv.Atoi(val); err == nil {
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
	return rq.innerHTML(rq.maybeClick(id, fn), "div", "", inner, attrs)
}

func (rq *Request) Span(id, inner string, fn ClickFn, attrs string) template.HTML {
	return rq.innerHTML(rq.maybeClick(id, fn), "span", "", inner, attrs)
}

func (rq *Request) Li(id, inner string, fn ClickFn, attrs string) template.HTML {
	return rq.innerHTML(rq.maybeClick(id, fn), "li", "", inner, attrs)
}

func (rq *Request) Td(id, inner string, fn ClickFn, attrs string) template.HTML {
	return rq.innerHTML(rq.maybeClick(id, fn), "td", "", inner, attrs)
}

func (rq *Request) A(id, inner string, fn ClickFn, attrs string) template.HTML {
	return rq.innerHTML(rq.maybeClick(id, fn), "a", "", inner, attrs)
}

func (rq *Request) Button(id, txt string, fn ClickFn, attrs string) template.HTML {
	return rq.innerHTML(rq.maybeClick(id, fn), "button", "button", txt, attrs)
}

func (rq *Request) Text(id, val string, fn InputTextFn, attrs string) template.HTML {
	return rq.inputHTML(rq.maybeInputText(id, fn), "text", val, attrs)
}

func (rq *Request) Password(id string, fn InputTextFn, attrs string) template.HTML {
	return rq.inputHTML(rq.maybeInputText(id, fn), "password", "", attrs)
}

func (rq *Request) Int(id string, val int, fn InputIntFn, attrs string) template.HTML {
	return rq.inputHTML(rq.maybeInputInt(id, fn), "number", strconv.Itoa(val), attrs)
}

func (rq *Request) Range(id string, val float64, fn InputFloatFn, attrs string) template.HTML {
	return rq.inputHTML(rq.maybeInputFloat(id, fn), "range", strconv.FormatFloat(val, 'f', -1, 64), attrs)
}

func (rq *Request) Checkbox(id string, val bool, fn InputBoolFn, attrs string) template.HTML {
	if val {
		attrs += " checked"
	}
	return rq.inputHTML(rq.maybeInputBool(id, fn), "checkbox", "", attrs)
}

func (rq *Request) Date(id string, val time.Time, fn InputDateFn, attrs string) template.HTML {
	if val.IsZero() {
		val = time.Now()
	}
	return rq.inputHTML(rq.maybeInputDate(id, fn), "date", val.Format(ISO8601), attrs)
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
	return rq.inputHTML(rq.maybeInputBool(id, fn), "radio", "", attrs)
}

func (rq *Request) Select(id string, val *NamedBoolArray, fn InputTextFn, attrs string) template.HTML {
	id = rq.maybeInputText(id, fn)
	b := make([]byte, 0, 256)
	b = append(b, "<select id=\""...)
	b = append(b, id...)
	b = append(b, '"')
	if attrs != "" {
		b = append(b, ' ')
		b = append(b, attrs...)
	}
	b = append(b, ">\n"...)
	for _, nb := range *val {
		b = append(b, "<option value=\""...)
		b = append(b, (nb.Value)...)
		if nb.Checked {
			b = append(b, "\" selected>"...)
		} else {
			b = append(b, "\">"...)
		}
		b = append(b, (nb.Text)...)
		b = append(b, "</option>\n"...)
	}
	b = append(b, "</select>\n"...)
	return template.HTML(b) // #nosec G203
}
