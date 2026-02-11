package jaws

import (
	"io"

	"github.com/linkdata/jaws/jid"
)

type registerUI struct {
	Updater
}

func (registerUI) JawsRender(*Element, io.Writer, []any) error {
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
func (rq RequestWriter) Register(updater Updater, params ...any) jid.Jid {
	elem := rq.rq.NewElement(registerUI{Updater: updater})
	elem.Tag(updater)
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	return elem.Jid()
}
