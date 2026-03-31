package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
)

type Tr struct{ HTMLInner }

func NewTr(innerHTML jawsbind.HTMLGetter) *Tr { return &Tr{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Tr(innerHTML any, params ...any) error {
	return rw.UI(NewTr(jawsbind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Tr) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "tr", "", params)
}
