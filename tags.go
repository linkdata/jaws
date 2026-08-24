package jaws

import (
	"html/template"

	"github.com/linkdata/jaws/lib/tag"
)

// eligibleAsTag reports whether t requires expansion to validate or passes validate.
func eligibleAsTag(t any, validate func(any) error) (ok bool) {
	if _, ok = t.(tag.TagGetter); ok {
		return true
	}
	switch t.(type) {
	case []any, []tag.Tag:
		return true
	default:
		return validate(t) == nil
	}
}

// ParseParams parses the parameters passed to UI helpers when creating a new
// [Element], returning UI tags, event handlers and HTML attributes.
//
// ParseParams recognizes values whose dynamic type is exactly [InputFn], and
// values implementing [InputHandler], [ClickHandler] or [ContextMenuHandler],
// as event handlers. It does not invoke
// [InitialHTMLAttrHandler.JawsInitialHTMLAttr]; implementing that interface does
// not affect parameter classification.
//
// A nil [InputFn] is ignored.
//
// A recognized event handler that is also usable as a tag is returned in both tags
// and handlers.
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
			if eligibleAsTag(data, tag.NewErrNotUsableAsTag) {
				tags = append(tags, data)
			}
		}
	}
	return
}

// MustTagExpand expands tagValue and reports expansion errors through [Jaws.MustLog].
//
// With a [Jaws.Logger] configured, the error is queued and MustTagExpand returns
// [github.com/linkdata/jaws/lib/tag.TagExpand]'s partial result. Without one,
// [Jaws.MustLog] panics, so the partial result never reaches the caller.
func (jw *Jaws) MustTagExpand(tagValue any) (result []any) {
	result, err := tag.TagExpand(tagValue)
	jw.MustLog(err)
	return
}
