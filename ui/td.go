package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
)

type Td struct{ HTMLInner }

func NewTd(innerHTML jawsbind.HTMLGetter) *Td { return &Td{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	return rw.UI(NewTd(jawsbind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Td) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}
