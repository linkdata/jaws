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

type clickHandlerWrapper struct{ ClickHandler }

func (chw clickHandlerWrapper) JawsEvent(*Element, what.What, string) error {
	return ErrEventUnhandled
}
