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

// ErrJsVarArgumentType is returned when [RequestWriter.JsVar] receives an
// argument that is neither a JaWS UI nor a [JsVarMaker].
var ErrJsVarArgumentType = errors.New("expected jaws.UI or JsVarMaker")

// ErrJsVarTooLarge reports that a client-writable JsVar grew past [MaxClientJsVarBytes].
//
// It is returned by [JsVar.JawsRender] when the serialized size of a JsVar that does
// not implement [PathSetter] exceeds the cap; the [Request] is aborted. See the
// [JsVar] SECURITY note.
var ErrJsVarTooLarge = errors.New("jsvar: serialized value exceeds MaxClientJsVarBytes")
