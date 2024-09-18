package jaws

import (
	"io"

	"github.com/linkdata/jaws/jid"
)

type UiRegister struct {
	Updater
}

func (ui UiRegister) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	return
}

// Register creates a new Element with the given Updater as a tag
// for dynamic updates. Additional tags may be provided in params.
//
// Returns a Jid, suitable for including as a HTML "id" attribute:
//
//	<div id="{{$.Register .MyUpdater}}">...</div>
func (rq RequestWriter) Register(updater Updater, params ...any) jid.Jid {
	elem := rq.rq.NewElement(UiRegister{Updater: updater})
	elem.Tag(updater)
	elem.ApplyParams(params)
	return elem.Jid()
}
