package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Button struct{ HTMLInner }

func NewButton(innerHTML bind.HTMLGetter) *Button {
	return &Button{HTMLInner{HTMLGetter: innerHTML}}
}
func (rw RequestWriter) Button(innerHTML any, params ...any) error {
	return rw.UI(NewButton(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Button) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "button", "button", params)
}
