package ui

import (
	"io"
	"time"

	"github.com/linkdata/jaws/core"
)

type Date struct{ InputDate }

func NewDate(g core.Setter[time.Time]) *Date { return &Date{InputDate{Setter: g}} }
func (rw RequestWriter) Date(value any, params ...any) error {
	return rw.UI(NewDate(core.MakeSetter[time.Time](value)), params...)
}

func (ui *Date) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderDateInput(e, w, "date", params...)
}
