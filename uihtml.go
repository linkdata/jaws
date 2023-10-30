package jaws

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws/what"
)

type UiHtml struct {
	Tag any
}

func (ui *UiHtml) parseGetter(e *Element, getter interface{}) {
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

func parseParams(elem *Element, params []interface{}) (attrs []string) {
	for i := range params {
		switch data := params[i].(type) {
		case template.HTML:
			attrs = append(attrs, string(data))
		case []template.HTML:
			for _, s := range data {
				attrs = append(attrs, string(s))
			}
		case string:
			attrs = append(attrs, data)
		case []string:
			attrs = append(attrs, data...)
		case EventFn:
			if data != nil {
				elem.handlers = append(elem.handlers, eventFnWrapper{data})
			}
		default:
			if h, ok := data.(ClickHandler); ok {
				elem.handlers = append(elem.handlers, clickHandlerWapper{h})
			}
			if h, ok := data.(EventHandler); ok {
				elem.handlers = append(elem.handlers, h)
			}
			elem.Tag(data)
		}
	}
	return
}

func (ui *UiHtml) parseParams(elem *Element, params []interface{}) (attrs []string) {
	attrs = parseParams(elem, params)
	return
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer, params []interface{}) (err error) {
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
