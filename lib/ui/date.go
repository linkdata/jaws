package ui

import (
	"io"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Date struct{ InputDate }

// NewDate returns a date input widget bound to g.
func NewDate(g bind.Setter[time.Time]) *Date { return &Date{InputDate{Setter: g}} }

func (ui *Date) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderDateInput(e, w, "date", params...)
}

func (rw RequestWriter) Date(value any, params ...any) error {
	return rw.UI(NewDate(bind.MakeSetter[time.Time](value)), params...)
}
