package ui

import (
	"io"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
)

type Date struct{ InputDate }

func NewDate(g jawsbind.Setter[time.Time]) *Date { return &Date{InputDate{Setter: g}} }
func (rw RequestWriter) Date(value any, params ...any) error {
	return rw.UI(NewDate(jawsbind.MakeSetter[time.Time](value)), params...)
}

func (ui *Date) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderDateInput(e, w, "date", params...)
}
