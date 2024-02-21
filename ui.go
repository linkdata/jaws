package jaws

import (
	"io"
)

// If any of these functions panic, the Request will be closed and the panic logged.
// Optionally you may also implement ClickHandler and/or EventHandler.
type UI interface {
	// JawsRender is called once per Element when rendering the initial webpage.
	// Do not call this yourself unless it's from within another JawsRender implementation.
	JawsRender(e *Element, w io.Writer, params []any) error

	// JawsUpdate is called for an Element that has been marked dirty to update it's HTML.
	// Do not call this yourself unless it's from within another JawsUpdate implementation.
	JawsUpdate(e *Element)
}
