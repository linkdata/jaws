package jaws

import "io"

type Renderer interface {
	// JawsRender is called once per Element when rendering the initial webpage.
	// Do not call this yourself unless it's from within another JawsRender implementation.
	JawsRender(e *Element, w io.Writer, params []any) error
}
