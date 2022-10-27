package jaws

import (
	"html/template"
)

type Ui interface {
	UiID() string                                    // Must return a unique HTML id or a blank string
	UiHTML(id string, attrs ...string) template.HTML // The id may have been generated if UiID() returned a blank string
	UiEvent(rq *Request, id, evt, val string) error  // See jaws.EventFn
}
