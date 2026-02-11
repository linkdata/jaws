package ui

import (
	"strconv"
)

// ErrMissingTemplate is returned when trying to render an undefined template by name.
var ErrMissingTemplate errMissingTemplate

type errMissingTemplate string

func (e errMissingTemplate) Error() string {
	return "missing template " + strconv.Quote(string(e))
}

func (errMissingTemplate) Is(target error) bool {
	return target == ErrMissingTemplate
}
