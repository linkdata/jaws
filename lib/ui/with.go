package ui

import "github.com/linkdata/jaws"

// With is passed as the data parameter when using [RequestWriter.Template],
// populated with all required members set.
type With struct {
	*jaws.Element     // the Element being rendered using a template
	RequestWriter     // the RequestWriter for nested UI helpers
	Dot           any // user data parameter
	// Auth is the authentication information from [jaws.Jaws.MakeAuth]. When
	// MakeAuth is nil it is a [jaws.DefaultAuth], whose IsAdmin returns true for
	// everyone — gating UI on {{if .Auth.IsAdmin}} is only safe once MakeAuth is
	// set. See [jaws.DefaultAuth] for the fail-open caveat.
	Auth jaws.Auth
}
