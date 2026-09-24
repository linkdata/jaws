package ui

import "errors"

// ErrIllegalJsVarName is returned when a JsVar name is missing, is not a
// string, has invalid syntax, or is reserved.
//
// Valid names begin with an ASCII letter, underscore, or dollar sign, and
// contain only ASCII letters, digits, underscores, and dollar signs. The exact
// name "__proto__" is reserved.
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

// ErrJsVarTooLarge reports a failed client-writable JsVar size check.
//
// [JSONSizeCheck] returns an error matching ErrJsVarTooLarge when the tentative
// value exceeds its configured maximum or cannot be marshaled. A matching error
// from [JsVar.ClientCheck] makes [JsVar.JawsInput] reject the write and return
// ErrJsVarTooLarge. It also aborts the associated [jaws.Request], when present;
// the request cancellation cause retains the detailed check error.
var ErrJsVarTooLarge = errors.New("jsvar: JSON size check failed")

// ErrIllegalJsVarPath reports that a JsVar path contains a protocol byte.
//
// [JsVar.JawsSetPath] returns it for a path containing a tab, newline, carriage
// return, or equals sign, without applying or broadcasting the change.
// [JsVar.JawsInput] applies the same check to incoming browser writes.
var ErrIllegalJsVarPath = errors.New("jsvar: path contains illegal protocol byte (tab, newline, carriage return or equals)")
