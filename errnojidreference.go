package jaws

import (
	"errors"
)

// ErrNoJidReference is returned when a jaws.Template is missing a reference to the Jid.
// The top level HTML tag should have the attribute "id" set to "$.Jid".
var ErrNoJidReference = errors.New("template has no reference to the Jid")
