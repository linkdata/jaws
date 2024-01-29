package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiHtml struct {
	Tag any
}

func (ui *UiHtml) parseGetter(e *Element, getter any) {
	if getter != nil {
		if tagger, ok := getter.(TagGetter); ok {
			ui.Tag = tagger.JawsGetTag(e.Request)
			if ch, ok := getter.(ClickHandler); ok {
				e.handlers = append(e.handlers, clickHandlerWapper{ch})
			}
			if eh, ok := getter.(EventHandler); ok {
				e.handlers = append(e.handlers, eh)
			}
		} else {
			ui.Tag = getter
		}
		e.Tag(ui.Tag)
	}
}

/*func (ui *UiHtml) parseParams(elem *Element, params []any) (attrs []string) {
	return elem.ParseParams(params)
}*/

func (ui *UiHtml) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	if h, ok := ui.Tag.(UI); ok {
		err = h.JawsRender(e, w, params)
	}
	return
}

func (ui *UiHtml) JawsUpdate(e *Element) {
	if h, ok := ui.Tag.(UI); ok {
		h.JawsUpdate(e)
	}
}

func (ui *UiHtml) JawsEvent(e *Element, wht what.What, val string) error {
	return callEventHandler(ui.Tag, e, wht, val)
}
