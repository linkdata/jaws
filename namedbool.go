package jaws

import (
	"fmt"
)

// NamedBool stores a named boolen value with a HTML representation.
type NamedBool struct {
	Name    string // name within the named bool set
	Html    string // HTML code used in select lists or labels
	Checked bool   // it's state
}

// String returns a string representation of the NamedBool suitable for debugging.
func (nb *NamedBool) String() string {
	return fmt.Sprintf("&{%q,%q,%v}", nb.Name, nb.Html, nb.Checked)
}
