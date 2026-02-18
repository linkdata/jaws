package ui

import "errors"

// ErrIllegalJsVarName is returned when a JsVar name is missing, not a string,
// or does not follow valid top-level identifier syntax.
var ErrIllegalJsVarName errIllegalJsVarName

type errIllegalJsVarName string

func (e errIllegalJsVarName) Error() string {
	if why := string(e); why != "" {
		return "illegal jsvar name: " + why
	}
	return "illegal jsvar name"
}

func (errIllegalJsVarName) Is(target error) bool {
	return target == ErrIllegalJsVarName
}

// ErrJsVarArgumentType is returned when RequestWriter.JsVar receives an
// argument that is neither a core.UI nor a JsVarMaker.
var ErrJsVarArgumentType = errors.New("expected core.UI or JsVarMaker")
