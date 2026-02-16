package core

import "html/template"

// shouldAutoTagHandler returns true if a handler value is safe to use as a tag.
// Non-comparable handlers are only auto-tagged when they provide an explicit TagGetter.
func shouldAutoTagHandler(handler any) any {
	if _, ok := handler.(TagGetter); ok || newErrNotComparable(handler) == nil {
		return handler
	}
	return nil
}

// ParseParams parses the parameters passed to UI() when creating a new Element,
// returning UI tags, event handlers and HTML attributes.
func ParseParams(params []any) (tags []any, handlers []EventHandler, attrs []string) {
	for i := range params {
		switch data := params[i].(type) {
		case template.HTMLAttr:
			attrs = append(attrs, string(data))
		case []template.HTMLAttr:
			for _, s := range data {
				attrs = append(attrs, string(s))
			}
		case string:
			attrs = append(attrs, data)
		case []string:
			attrs = append(attrs, data...)
		case EventFn:
			if data != nil {
				handlers = append(handlers, eventFnWrapper{data})
			}
		default:
			if h, ok := data.(EventHandler); ok {
				handlers = append(handlers, h)
				data = shouldAutoTagHandler(data)
			} else if h, ok := data.(ClickHandler); ok {
				handlers = append(handlers, clickHandlerWrapper{h})
				data = shouldAutoTagHandler(data)
			}
			tags = append(tags, data)
		}
	}
	return
}
