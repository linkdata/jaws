package jaws

import (
	"io"
)

// UI defines the required methods on JaWS UI objects.
// In addition, all UI objects must be comparable so they can be used as map keys.
type UI interface {
	// JawsRender is called once per Element when rendering the initial webpage.
	// Do not call this yourself unless it's from within another JawsRender implementation.
	JawsRender(e *Element, w io.Writer, params []any) error

	// JawsUpdate is called for an Element that has been marked dirty to update it's HTML.
	// Do not call this yourself unless it's from within another JawsUpdate implementation.
	JawsUpdate(e *Element)
}
