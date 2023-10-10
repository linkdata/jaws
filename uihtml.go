package jaws

import (
	"html/template"
	"io"
	"strconv"
	"time"

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
		case func(*Request, string) error: // Deprecated: ClickFn
			if data != nil {
				elem.handlers = append(elem.handlers, eventFnWrapper{
					func(e *Element, wht what.What, val string) (err error) {
						if wht == what.Click {
							err = data(e.Request, e.jid.String())
						}
						return
					}})
			}
		case func(*Request, string, string) error: // Deprecated: InputTextFn
			if data != nil {
				elem.handlers = append(elem.handlers, eventFnWrapper{func(e *Element, wht what.What, val string) (err error) {
					if wht == what.Input {
						err = data(e.Request, e.jid.String(), val)
					}
					return
				}})
			}
		case func(*Request, string, bool) error: // Deprecated: InputBoolFn
			if data != nil {
				elem.handlers = append(elem.handlers, eventFnWrapper{func(e *Element, wht what.What, val string) (err error) {
					if wht == what.Input {
						var v bool
						if val != "" {
							if v, err = strconv.ParseBool(val); err != nil {
								return
							}
						}
						err = data(e.Request, e.jid.String(), v)
					}
					return
				}})
			}
		case func(*Request, string, float64) error: // Deprecated: InputFloatFn
			if data != nil {
				elem.handlers = append(elem.handlers, eventFnWrapper{func(e *Element, wht what.What, val string) (err error) {
					if wht == what.Input {
						var v float64
						if val != "" {
							if v, err = strconv.ParseFloat(val, 64); err != nil {
								return
							}
						}
						err = data(e.Request, e.jid.String(), v)
					}
					return
				}})
			}
		case func(*Request, string, time.Time) error: // Deprecated: InputDateFn
			if data != nil {
				elem.handlers = append(elem.handlers, eventFnWrapper{func(e *Element, wht what.What, val string) (err error) {
					if wht == what.Input {
						var v time.Time
						if val != "" {
							if v, err = time.Parse(ISO8601, val); err != nil {
								return
							}
						}
						err = data(e.Request, e.jid.String(), v)
					}
					return
				}})
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

func (ui *UiHtml) JawsRender(e *Element, w io.Writer, params []interface{}) {
	if h, ok := ui.Tag.(UI); ok {
		h.JawsRender(e, w, params)
	}
}

func (ui *UiHtml) JawsUpdate(e *Element) {
	if h, ok := ui.Tag.(UI); ok {
		h.JawsUpdate(e)
	}
}

func (ui *UiHtml) JawsEvent(e *Element, wht what.What, val string) error {
	return callEventHandler(ui.Tag, e, wht, val)
}
