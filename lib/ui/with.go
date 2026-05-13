package ui

import "github.com/linkdata/jaws"

// With is passed as the data parameter when using [RequestWriter.Template],
// populated with all required members set.
type With struct {
	*jaws.Element           // the Element being rendered using a template
	RequestWriter           // the RequestWriter for nested UI helpers
	Dot           any       // user data parameter
	Auth          jaws.Auth // optional authentication information returned by MakeAuthFn
}
