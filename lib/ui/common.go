package ui

import (
	"errors"

	"github.com/linkdata/jaws"
)

func applyDirty(tag any, elem *jaws.Element, err error) (retErr error) {
	if !errors.Is(err, jaws.ErrValueUnchanged) {
		retErr = err
		elem.Dirty(tag)
	}
	return
}
