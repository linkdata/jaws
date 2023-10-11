package jaws

import "github.com/linkdata/jaws/what"

type ClickHandler interface {
	JawsClick(e *Element, name string) (stop bool, err error)
}

type clickHandlerWapper struct{ ClickHandler }

func (chw clickHandlerWapper) JawsEvent(e *Element, w what.What, v string) (stop bool, err error) {
	if w == what.Click {
		return chw.JawsClick(e, v)
	}
	return
}
