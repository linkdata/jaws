package bind

import (
	"html/template"

	"github.com/linkdata/jaws"
)

// A HTMLGetter is the primary way to deliver generated HTML content to dynamic HTML nodes.
type HTMLGetter interface {
	JawsGetHTML(e *jaws.Element) template.HTML
}
