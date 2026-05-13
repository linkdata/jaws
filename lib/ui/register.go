package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jid"
)

// Register creates an element used for update-only registration.
type Register struct{ jaws.Updater }

// NewRegister returns an update-only widget that invokes updater during updates.
func NewRegister(updater jaws.Updater) Register { return Register{Updater: updater} }

// JawsRender renders no HTML for update-only registration.
func (u Register) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return nil
}

// Register creates a new Element with the given Updater as a tag
// for dynamic updates. Additional tags may be provided in params.
// The updater's [jaws.Updater.JawsUpdate] method will be called immediately to
// ensure the initial rendering is correct.
//
// Returns a [jid.Jid], suitable for including as an HTML id attribute:
//
//	<div id="{{$.Register .MyUpdater}}">...</div>
func (rw RequestWriter) Register(updater jaws.Updater, params ...any) jid.Jid {
	elem := rw.NewElement(Register{Updater: updater})
	elem.Tag(updater)
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	return elem.Jid()
}
