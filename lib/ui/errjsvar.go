package ui

import "errors"

// ErrIllegalJsVarName reports an invalid or reserved JsVar name.
//
// A valid name is a non-empty top-level JavaScript identifier other than the
// reserved browser routing name "__proto__".
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
// not implement [PathSetter] exceeds the cap; the [jaws.Request] is aborted. See the
// [JsVar] SECURITY note.
var ErrJsVarTooLarge = errors.New("jsvar: serialized value exceeds MaxClientJsVarBytes")

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
