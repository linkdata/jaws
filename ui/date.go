package ui

import (
	"io"
	"time"

	pkg "github.com/linkdata/jaws/jaws"
)

type Date struct{ InputDate }

func NewDate(g pkg.Setter[time.Time]) *Date { return &Date{InputDate{Setter: g}} }
func (ui *Date) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderDateInput(e, w, "date", params...)
}
