package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Td struct{ HTMLInner }

func NewTd(innerHTML bind.HTMLGetter) *Td { return &Td{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	return rw.UI(NewTd(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Td) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}
