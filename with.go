package jaws

import (
	"html/template"
)

// With is passed as the data parameter when using RequestWriter.Template(),
// populated with all required members set.
type With struct {
	*Element                        // the Element being rendered using a template.
	RequestWriter                   // the RequestWriter
	Dot           any               // user data parameter
	Attrs         template.HTMLAttr // HTML attributes string
	Auth          Auth              // (optional) authentication information returned by MakeAuthFn
}
