package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Tr renders an HTML table row with dynamic inner HTML.
type Tr struct{ HTMLInner }

// NewTr returns a table row widget whose inner HTML is rendered from innerHTML.
//
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewTr(innerHTML any) *Tr { return &Tr{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

// JawsRender renders ui as an HTML table row.
func (u *Tr) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "tr", "", params)
}

// Tr renders an HTML table row.
//
// A plain string innerHTML is trusted HTML and is not escaped; see [NewTr] and
// [bind.MakeHTMLGetter] for how to pass untrusted user input safely.
func (rw RequestWriter) Tr(innerHTML any, params ...any) error {
	return rw.NewUI(NewTr(innerHTML), params...)
}
