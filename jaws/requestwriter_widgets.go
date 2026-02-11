package jaws

import "time"

// RequestWriterWidgetFactory provides widget constructors used by
// RequestWriter helper methods (A, Span, Text, ...).
//
// The ui package registers the canonical implementation via init(), so core
// RequestWriter keeps its legacy API without duplicating widget logic.
type RequestWriterWidgetFactory struct {
	A         func(HTMLGetter) UI
	Button    func(HTMLGetter) UI
	Checkbox  func(Setter[bool]) UI
	Container func(string, Container) UI
	Date      func(Setter[time.Time]) UI
	Div       func(HTMLGetter) UI
	Img       func(Getter[string]) UI
	Label     func(HTMLGetter) UI
	Li        func(HTMLGetter) UI
	Number    func(Setter[float64]) UI
	Password  func(Setter[string]) UI
	Radio     func(Setter[bool]) UI
	Range     func(Setter[float64]) UI
	Select    func(SelectHandler) UI
	Span      func(HTMLGetter) UI
	Tbody     func(Container) UI
	Td        func(HTMLGetter) UI
	Text      func(Setter[string]) UI
	Textarea  func(Setter[string]) UI
	Tr        func(HTMLGetter) UI
}

var requestWriterWidgets RequestWriterWidgetFactory

// RegisterRequestWriterWidgets installs constructor hooks for RequestWriter
// helper methods. This is called by package ui during init().
func RegisterRequestWriterWidgets(f RequestWriterWidgetFactory) {
	requestWriterWidgets = f
}

func mustRequestWriterWidgets() RequestWriterWidgetFactory {
	f := requestWriterWidgets
	if f.Span == nil {
		panic("jaws: RequestWriter widget helpers are not registered; import github.com/linkdata/jaws/ui or github.com/linkdata/jaws")
	}
	return f
}

// A writes an <a> element.
func (rw RequestWriter) A(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.A(MakeHTMLGetter(innerHTML)), params...)
}

// Button writes a <button type="button"> element.
func (rw RequestWriter) Button(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Button(MakeHTMLGetter(innerHTML)), params...)
}

// Checkbox writes an <input type="checkbox"> element.
func (rw RequestWriter) Checkbox(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Checkbox(makeSetter[bool](value)), params...)
}

// Container writes a dynamic container element with the given HTML tag.
func (rw RequestWriter) Container(outerHTMLTag string, c Container, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Container(outerHTMLTag, c), params...)
}

// Date writes an <input type="date"> element.
func (rw RequestWriter) Date(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Date(makeSetter[time.Time](value)), params...)
}

// Div writes a <div> element.
func (rw RequestWriter) Div(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Div(MakeHTMLGetter(innerHTML)), params...)
}

// Img writes an <img> element.
func (rw RequestWriter) Img(imageSrc any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Img(makeGetter[string](imageSrc)), params...)
}

// Label writes a <label> element.
func (rw RequestWriter) Label(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Label(MakeHTMLGetter(innerHTML)), params...)
}

// Li writes a <li> element.
func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Li(MakeHTMLGetter(innerHTML)), params...)
}

// Number writes an <input type="number"> element.
func (rw RequestWriter) Number(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Number(makeSetterFloat64(value)), params...)
}

// Password writes an <input type="password"> element.
func (rw RequestWriter) Password(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Password(makeSetter[string](value)), params...)
}

// Radio writes an <input type="radio"> element.
func (rw RequestWriter) Radio(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Radio(makeSetter[bool](value)), params...)
}

// Range writes an <input type="range"> element.
func (rw RequestWriter) Range(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Range(makeSetterFloat64(value)), params...)
}

// Select writes a <select> element.
func (rw RequestWriter) Select(sh SelectHandler, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Select(sh), params...)
}

// Span writes a <span> element.
func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Span(MakeHTMLGetter(innerHTML)), params...)
}

// Tbody writes a <tbody> element.
func (rw RequestWriter) Tbody(c Container, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Tbody(c), params...)
}

// Td writes a <td> element.
func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Td(MakeHTMLGetter(innerHTML)), params...)
}

// Text writes an <input type="text"> element.
func (rw RequestWriter) Text(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Text(makeSetter[string](value)), params...)
}

// Textarea writes a <textarea> element.
func (rw RequestWriter) Textarea(value any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Textarea(makeSetter[string](value)), params...)
}

// Tr writes a <tr> element.
func (rw RequestWriter) Tr(innerHTML any, params ...any) error {
	f := mustRequestWriterWidgets()
	return rw.UI(f.Tr(MakeHTMLGetter(innerHTML)), params...)
}
