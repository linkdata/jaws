package jaws

import "html/template"

// ParseParams parses the parameters passed to UI() when creating a new Element,
// returning UI tags, event handlers and HTML attributes.
func ParseParams(params []any) (tags []any, handlers []EventHandler, attrs []template.HTMLAttr) {
	for i := range params {
		switch data := params[i].(type) {
		case template.HTMLAttr:
			attrs = append(attrs, data)
		case []template.HTMLAttr:
			attrs = append(attrs, data...)
		case string:
			attr := template.HTMLAttr(data) // #nosec G203
			attrs = append(attrs, attr)
		case []string:
			for _, s := range data {
				attr := template.HTMLAttr(s) // #nosec G203
				attrs = append(attrs, attr)
			}
		case EventFn:
			if data != nil {
				handlers = append(handlers, eventFnWrapper{data})
			}
		default:
			if h, ok := data.(ClickHandler); ok {
				handlers = append(handlers, clickHandlerWapper{h})
			}
			if h, ok := data.(EventHandler); ok {
				handlers = append(handlers, h)
			}
			tags = append(tags, data)
		}
	}
	return
}
