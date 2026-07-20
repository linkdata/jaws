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

// ErrIllegalJsVarPath reports that a JsVar path contained a protocol byte.
//
// A JsVar path is written verbatim into a what.Set frame (only the value side is
// JSON-encoded), and the client splits frames on '\n', fields on '\t', and the
// JsVar payload at the first '='. A path carrying those bytes could corrupt the
// frame, inject fabricated orders, or make peer browsers parse the value as
// invalid JSON, so [JsVar.JawsSetPath] rejects it before applying or
// broadcasting. [JsVar.JawsInput] applies the same check to the parsed path for
// incoming browser writes. The raw path is deliberately not echoed in the message
// to avoid log injection.
var ErrIllegalJsVarPath = errors.New("jsvar: path contains illegal protocol byte (tab, newline, carriage return or equals)")
