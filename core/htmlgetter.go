package core

import (
	"html/template"
)

// A HTMLGetter is the primary way to deliver generated HTML content to dynamic HTML nodes.
type HTMLGetter interface {
	JawsGetHTML(e *Element) template.HTML
}
