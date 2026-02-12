package core

import "github.com/linkdata/jaws/what"

type ClickHandler interface {
	// JawsClick is called when an Element's HTML element or something within it
	// is clicked in the browser.
	//
	// The name parameter is taken from the first 'name' HTML attribute or HTML
	// 'button' textContent found when traversing the DOM. It may be empty.
	JawsClick(e *Element, name string) (err error)
}

type clickHandlerWapper struct{ ClickHandler }

func (chw clickHandlerWapper) JawsEvent(e *Element, w what.What, v string) error {
	if w == what.Click {
		return chw.JawsClick(e, v)
	}
	return ErrEventUnhandled
}
