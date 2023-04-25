package jaws

import (
	"html/template"
	"strconv"
	"time"
)

type ClickFn func(rq *Request) error
type InputTextFn func(rq *Request, val string) error
type InputFloatFn func(rq *Request, val float64) error
type InputBoolFn func(rq *Request, val bool) error
type InputDateFn func(rq *Request, val time.Time) error

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
	rq.maybeEvent(jid, "trigger", fn)
	return nil
}

func (rq *Request) Div(jid, inner string, fn ClickFn, attrs ...string) template.HTML {
	return HtmlInner(rq.maybeClick(jid, fn), "div", "", inner, attrs...)
}

func (rq *Request) Span(jid, inner string, fn ClickFn, attrs ...string) template.HTML {
	return HtmlInner(rq.maybeClick(jid, fn), "span", "", inner, attrs...)
}

func (rq *Request) Li(jid, inner string, fn ClickFn, attrs ...string) template.HTML {
	return HtmlInner(rq.maybeClick(jid, fn), "li", "", inner, attrs...)
}

func (rq *Request) Td(jid, inner string, fn ClickFn, attrs ...string) template.HTML {
	return HtmlInner(rq.maybeClick(jid, fn), "td", "", inner, attrs...)
}

func (rq *Request) A(jid, inner string, fn ClickFn, attrs ...string) template.HTML {
	return HtmlInner(rq.maybeClick(jid, fn), "a", "", inner, attrs...)
}

func (rq *Request) Button(jid, txt string, fn ClickFn, attrs ...string) template.HTML {
	return HtmlInner(rq.maybeClick(jid, fn), "button", "button", txt, attrs...)
}

func (rq *Request) Img(jid, src string, fn ClickFn, attrs ...string) template.HTML {
	if src != "" && src[0] == '"' {
		src = `src=` + src
	} else {
		src = `src="` + src + `"`
	}
	attrs = append(attrs, src)
	return HtmlInner(rq.maybeClick(jid, fn), "img", "", "", attrs...)
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

func (rq *Request) Select(nba *NamedBoolArray, fn InputTextFn, attrs ...string) template.HTML {
	return HtmlSelect(rq.maybeInputText(nba.Jid, fn), nba, attrs...)
}

func (rq *Request) Ui(elem Ui, attrs ...string) template.HTML {
	return elem.JawsUi(rq, attrs...)
}

func (rq *Request) RadioGroup(nba *NamedBoolArray, fn InputTextFn) {
	rq.maybeInputText(nba.Jid, fn)
}

func (rq *Request) radioLocked(nba *NamedBoolArray, id, jid string, attrs []string) template.HTML {
	attrs = append(attrs, "name=\""+nba.Jid+"\"")
	if id != "" {
		attrs = append(attrs, "id=\""+id+"\"")
	}
	if nba.isCheckedLocked(jid) {
		attrs = append(attrs, "checked")
	}
	return HtmlInput(jid, "radio", "", attrs...)
}

func (rq *Request) Radio(nba *NamedBoolArray, jid string, attrs ...string) template.HTML {
	nba.mu.RLock()
	defer nba.mu.RUnlock()
	return rq.radioLocked(nba, "", jid, attrs)
}

func (rq *Request) LabeledRadioGroup(nba *NamedBoolArray, fn InputTextFn, radioAttrs, labelAttrs []string) template.HTML {
	b := make([]byte, 0)
	rq.RadioGroup(nba, fn)
	nba.ReadLocked(func(nbl []*NamedBool) {
		for _, nb := range nbl {
			htmlId := nba.Jid + "/" + nb.Name
			b = append(b, rq.radioLocked(nba, htmlId, nb.Name, radioAttrs)...)
			b = append(b, `<label for="`...)
			b = append(b, []byte(htmlId)...)
			b = append(b, '"')
			for _, attr := range labelAttrs {
				b = append(b, ' ')
				b = append(b, attr...)
			}
			b = append(b, '>')
			b = append(b, []byte(nb.Text)...)
			b = append(b, `</label>`...)
		}
	})
	return template.HTML(b) // #nosec G203
}
