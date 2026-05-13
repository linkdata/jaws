package ui

import "github.com/linkdata/jaws"

func applyDirty(tag any, elem *jaws.Element, err error) (retErr error) {
	if err != jaws.ErrValueUnchanged {
		retErr = err
		elem.Dirty(tag)
	}
	return
}
