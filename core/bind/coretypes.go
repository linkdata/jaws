package bind

import core "github.com/linkdata/jaws/core"

type (
	// Element is an alias for core.Element.
	Element = core.Element
	// ClickHandler is an alias for core.ClickHandler.
	ClickHandler = core.ClickHandler
)

var (
	// ErrEventUnhandled is returned by event handlers to pass handling onward.
	ErrEventUnhandled = core.ErrEventUnhandled
	// ErrValueUnchanged indicates a setter call changed nothing.
	ErrValueUnchanged = core.ErrValueUnchanged
)
