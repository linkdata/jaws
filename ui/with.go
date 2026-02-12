package ui

import (
	"html/template"

	"github.com/linkdata/jaws/core"
)

// With is passed as the data parameter when using RequestWriter.Template(),
// populated with all required members set.
type With struct {
	*core.Element                   // the Element being rendered using a template.
	RequestWriter                   // the RequestWriter
	Dot           any               // user data parameter
	Attrs         template.HTMLAttr // HTML attributes string
	Auth          core.Auth         // (optional) authentication information returned by MakeAuthFn
}
