package ui

import (
	"io"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Date renders an HTML date input bound to a time value setter.
type Date struct{ InputDate }

// NewDate returns a date input widget bound to g.
func NewDate(g bind.Setter[time.Time]) *Date { return &Date{InputDate{Setter: g}} }

// JawsRender renders ui as an HTML date input.
func (u *Date) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderDateInput(elem, w, "date", params...)
}

// Date renders an HTML date input.
func (rw RequestWriter) Date(value any, params ...any) error {
	return rw.UI(NewDate(bind.MakeSetter[time.Time](value)), params...)
}
