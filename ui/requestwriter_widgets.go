package ui

import (
	"time"

	"github.com/linkdata/jaws/core"
)

// A writes an <a> element.
func (rw RequestWriter) A(innerHTML any, params ...any) error {
	return rw.UI(NewA(core.MakeHTMLGetter(innerHTML)), params...)
}

// Button writes a <button type="button"> element.
func (rw RequestWriter) Button(innerHTML any, params ...any) error {
	return rw.UI(NewButton(core.MakeHTMLGetter(innerHTML)), params...)
}

// Checkbox writes an <input type="checkbox"> element.
func (rw RequestWriter) Checkbox(value any, params ...any) error {
	return rw.UI(NewCheckbox(core.MakeSetter[bool](value)), params...)
}

// Container writes a dynamic container element with the given HTML tag.
func (rw RequestWriter) Container(outerHTMLTag string, c core.Container, params ...any) error {
	return rw.UI(NewContainer(outerHTMLTag, c), params...)
}

// Date writes an <input type="date"> element.
func (rw RequestWriter) Date(value any, params ...any) error {
	return rw.UI(NewDate(core.MakeSetter[time.Time](value)), params...)
}

// Div writes a <div> element.
func (rw RequestWriter) Div(innerHTML any, params ...any) error {
	return rw.UI(NewDiv(core.MakeHTMLGetter(innerHTML)), params...)
}

// Img writes an <img> element.
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	return rw.UI(NewImg(core.MakeGetter[string](imageSrc)), params...)
}

// Label writes a <label> element.
func (rw RequestWriter) Label(innerHTML any, params ...any) error {
	return rw.UI(NewLabel(core.MakeHTMLGetter(innerHTML)), params...)
}

// Li writes a <li> element.
func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	return rw.UI(NewLi(core.MakeHTMLGetter(innerHTML)), params...)
}

// Number writes an <input type="number"> element.
func (rw RequestWriter) Number(value any, params ...any) error {
	return rw.UI(NewNumber(core.MakeSetterFloat64(value)), params...)
}

// Password writes an <input type="password"> element.
func (rw RequestWriter) Password(value any, params ...any) error {
	return rw.UI(NewPassword(core.MakeSetter[string](value)), params...)
}

// Radio writes an <input type="radio"> element.
func (rw RequestWriter) Radio(value any, params ...any) error {
	return rw.UI(NewRadio(core.MakeSetter[bool](value)), params...)
}

// Range writes an <input type="range"> element.
func (rw RequestWriter) Range(value any, params ...any) error {
	return rw.UI(NewRange(core.MakeSetterFloat64(value)), params...)
}

// Select writes a <select> element.
func (rw RequestWriter) Select(sh core.SelectHandler, params ...any) error {
	return rw.UI(NewSelect(sh), params...)
}

// Span writes a <span> element.
func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	return rw.UI(NewSpan(core.MakeHTMLGetter(innerHTML)), params...)
}

// Tbody writes a <tbody> element.
func (rw RequestWriter) Tbody(c core.Container, params ...any) error {
	return rw.UI(NewTbody(c), params...)
}

// Td writes a <td> element.
func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	return rw.UI(NewTd(core.MakeHTMLGetter(innerHTML)), params...)
}

// Text writes an <input type="text"> element.
func (rw RequestWriter) Text(value any, params ...any) error {
	return rw.UI(NewText(core.MakeSetter[string](value)), params...)
}

// Textarea writes a <textarea> element.
func (rw RequestWriter) Textarea(value any, params ...any) error {
	return rw.UI(NewTextarea(core.MakeSetter[string](value)), params...)
}

// Tr writes a <tr> element.
func (rw RequestWriter) Tr(innerHTML any, params ...any) error {
	return rw.UI(NewTr(core.MakeHTMLGetter(innerHTML)), params...)
}
