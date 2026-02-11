package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/jid"
)

// Register creates an element used for update-only registration.
type Register struct{ core.Updater }

func NewRegister(updater core.Updater) Register { return Register{Updater: updater} }
func (ui Register) JawsRender(*core.Element, io.Writer, []any) error {
	return nil
}

// Register creates a new Element with the given Updater as a tag
// for dynamic updates. Additional tags may be provided in params.
// The updaters JawsUpdate method will be called immediately to
// ensure the initial rendering is correct.
//
// Returns a Jid, suitable for including as a HTML "id" attribute:
//
//	<div id="{{$.Register .MyUpdater}}">...</div>
func (rqw RequestWriter) Register(updater core.Updater, params ...any) jid.Jid {
	elem := rqw.NewElement(Register{Updater: updater})
	elem.Tag(updater)
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	return elem.Jid()
}
