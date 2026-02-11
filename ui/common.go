package ui

import pkg "github.com/linkdata/jaws/jaws"

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func applyDirty(tag any, e *pkg.Element, err error) (changed bool, retErr error) {
	switch err {
	case nil:
		e.Dirty(tag)
		return true, nil
	case pkg.ErrValueUnchanged:
		return false, nil
	default:
		return false, err
	}
}
