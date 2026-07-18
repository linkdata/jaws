package ui

import (
	"io"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Date renders an HTML date input bound to a time value setter.
//
// The control is date-only: a browser edit normalizes the bound [time.Time] to
// midnight UTC of the picked date, discarding time-of-day and location. See
// [InputDate.JawsInput].
type Date struct{ InputDate }

// NewDate returns a date input widget bound to g.
//
// The widget is date-only; see [InputDate.JawsInput] for how a browser edit
// normalizes the bound [time.Time] to midnight UTC.
func NewDate(g bind.Setter[time.Time]) *Date { return &Date{InputDate{Setter: g}} }

// JawsRender renders ui as an HTML date input.
func (u *Date) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderDateInput(elem, w, "date", params...)
}

// Date renders an HTML date input.
//
// The control is date-only: a browser edit normalizes the bound [time.Time] to
// midnight UTC of the picked date, discarding time-of-day and location. See
// [InputDate.JawsInput].
func (rw RequestWriter) Date(value any, params ...any) error {
	return rw.NewUI(NewDate(bind.MakeSetter[time.Time](value)), params...)
}
