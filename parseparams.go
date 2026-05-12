package jaws

import (
	"html/template"

	"github.com/linkdata/jaws/lib/tag"
)

// usableAsTag returns true if it is safe to use as a tag, false otherwise.
func usableAsTag(t any) (ok bool) {
	_, ok = t.(tag.TagGetter)
	return ok || tag.NewErrNotComparable(t) == nil
}

// ParseParams parses the parameters passed to UI helpers when creating a new
// [Element], returning UI tags, event handlers and HTML attributes.
func ParseParams(params []any) (tags []any, handlers []any, attrs []string) {
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
		case InputFn:
			if data != nil {
				handlers = append(handlers, data)
			}
		default:
			if _, ok := data.(InputHandler); ok {
				handlers = append(handlers, data)
			} else if _, ok := data.(ClickHandler); ok {
				handlers = append(handlers, data)
			} else if _, ok := data.(ContextMenuHandler); ok {
				handlers = append(handlers, data)
			}
			if usableAsTag(data) {
				tags = append(tags, data)
			}
		}
	}
	return
}
