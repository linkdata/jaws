package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

// Register creates an element used for update-only registration.
type Register struct{ pkg.Updater }

func NewRegister(updater pkg.Updater) Register { return Register{Updater: updater} }
func (ui Register) JawsRender(*pkg.Element, io.Writer, []any) error {
	return nil
}
