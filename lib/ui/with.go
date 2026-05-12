package ui

import (
	"html/template"

	"github.com/linkdata/jaws"
)

// With is passed as the data parameter when using [RequestWriter.Template],
// populated with all required members set.
type With struct {
	*jaws.Element                   // the Element being rendered using a template
	RequestWriter                   // the RequestWriter for nested UI helpers
	Dot           any               // user data parameter
	Attrs         template.HTMLAttr // HTML attributes string
	Auth          jaws.Auth         // (optional) authentication information returned by MakeAuthFn
}
