package jawsbind

import "github.com/linkdata/jaws"

type (
	// Element is an alias for jaws.Element.
	Element = jaws.Element
	// ClickHandler is an alias for jaws.ClickHandler.
	ClickHandler = jaws.ClickHandler
)

var (
	// ErrEventUnhandled is returned by event handlers to pass handling onward.
	ErrEventUnhandled = jaws.ErrEventUnhandled
	// ErrValueUnchanged indicates a setter call changed nothing.
	ErrValueUnchanged = jaws.ErrValueUnchanged
)
