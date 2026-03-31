package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type Span struct{ HTMLInner }

func NewSpan(innerHTML bind.HTMLGetter) *Span { return &Span{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	return rw.UI(NewSpan(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Span) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}
