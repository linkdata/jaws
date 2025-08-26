package jaws

type Updater interface {
	// JawsUpdate is called for an Element that has been marked dirty to update it's HTML.
	// Do not call this yourself unless it's from within another JawsUpdate implementation.
	JawsUpdate(e *Element)
}
