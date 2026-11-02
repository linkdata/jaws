package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
)

// File renders a named native file input for a multipart form.
//
// The zero value renders an input with an empty name, which browsers omit from
// form submission.
//
// The browser submits selected files through the owning form's HTTP request;
// file data and names are not sent through JaWS input events.
// [File.JawsUpdate] leaves the existing input untouched; replacing it clears
// its selection.
type File struct {
	name           string
	submitOnChange bool
}

// NewFile returns a file input with the given form field name.
//
// If submitOnChange is true, selecting at least one file calls the owning
// form's requestSubmit method. The browser validates the form and, if valid,
// fires its submit event.
// An empty selection or an input without an owning form does nothing.
func NewFile(name string, submitOnChange bool) *File {
	return &File{name: name, submitOnChange: submitOnChange}
}

// JawsRender renders the file input without a value attribute.
func (u *File) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	attrs := []template.HTMLAttr{htmlio.Attr("name", u.name)}
	if u.submitOnChange {
		attrs = append(attrs, "data-jawsonchangesubmit")
	}
	attrs = append(attrs, elem.ApplyParams(params)...)
	return htmlio.WriteHTMLInput(w, elem.Jid(), "file", "", attrs)
}

// JawsUpdate leaves the browser's selected files unchanged.
func (u *File) JawsUpdate(*jaws.Element) {}

// File renders a named native file input for a multipart form.
//
// Set submitOnChange to submit the owning form when files are selected.
// Use an accept attribute in params, such as `accept=".png,.jpg"`, to hint
// which file types the picker should show. Use [htmlio.Attr] for dynamic
// attribute values. The HTTP handler must protect against CSRF, authorize the
// request, validate uploaded files, and enforce size limits.
func (rw RequestWriter) File(name string, submitOnChange bool, params ...any) error {
	return rw.NewUI(NewFile(name, submitOnChange), params...)
}
