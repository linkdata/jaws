package ui

import "github.com/linkdata/jaws/core"

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func applyDirty(tag any, e *core.Element, err error) (changed bool, retErr error) {
	switch err {
	case nil:
		e.Dirty(tag)
		return true, nil
	case core.ErrValueUnchanged:
		return false, nil
	default:
		return false, err
	}
}
