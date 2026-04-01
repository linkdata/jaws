package ui

import "github.com/linkdata/jaws"

func applyDirty(tag any, e *jaws.Element, err error) (retErr error) {
	if err != jaws.ErrValueUnchanged {
		retErr = err
		e.Dirty(tag)
	}
	return
}
