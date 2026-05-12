package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Li struct{ HTMLInner }

func NewLi(innerHTML any) *Li { return &Li{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }
func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	return rw.UI(NewLi(innerHTML), params...)
}

func (ui *Li) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}
