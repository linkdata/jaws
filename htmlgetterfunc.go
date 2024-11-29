package jaws

import "html/template"

type htmlGetterFunc struct {
	fn func(*Element) template.HTML
}

func (g htmlGetterFunc) JawsGetHtml(e *Element) template.HTML {
	return g.fn(e)
}

// HtmlGetterFunc wraps a function and returns a HtmlGetter.
func HtmlGetterFunc(fn func(*Element) template.HTML) HtmlGetter {
	return htmlGetterFunc{fn: fn}
}
