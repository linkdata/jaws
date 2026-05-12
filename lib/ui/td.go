package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Td struct{ HTMLInner }

func NewTd(innerHTML any) *Td { return &Td{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }
func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	return rw.UI(NewTd(innerHTML), params...)
}

func (ui *Td) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}
