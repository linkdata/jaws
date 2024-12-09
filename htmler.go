package jaws

import (
	"fmt"
	"html"
	"html/template"
	"sync"
)

type htmler struct {
	l    sync.Locker
	f    string
	args []any
}

// JawsGetHtml implements HtmlGetter.
func (h htmler) JawsGetHtml(e *Element) template.HTML {
	if rl, ok := h.l.(RWLocker); ok {
		rl.RLock()
		defer rl.RUnlock()
	} else {
		h.l.Lock()
		defer h.l.Unlock()
	}
	var args []any
	for _, arg := range h.args {
		switch x := arg.(type) {
		case string:
			arg = html.EscapeString(x)
		case fmt.Stringer:
			arg = html.EscapeString(x.String())
		}
		args = append(args, arg)
	}
	return template.HTML( /*#nosec G203*/ fmt.Sprintf(h.f, args...))
}

func (h htmler) JawsGetTag(*Request) any {
	var tags []any
	for _, arg := range h.args {
		if _, ok := arg.(fmt.Stringer); ok {
			tags = append(tags, arg)
		}
	}
	return tags
}

// HTMLer return a lock protected jaws.HtmlGetter using the given formatting
// and arguments. Arguments of type string or fmt.Stringer will be escaped
// using html.EscapeString().
//
// Returns all fmt.Stringer arguments as UI tags.
func HTMLer(l sync.Locker, formatting string, args ...any) HtmlGetter {
	return htmler{l, formatting, args}
}
