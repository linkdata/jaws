package ui

import core "github.com/linkdata/jaws/core"

func applyDirty(tag any, e *core.Element, err error) (retErr error) {
	if err != core.ErrValueUnchanged {
		retErr = err
		e.Dirty(tag)
	}
	return
}
