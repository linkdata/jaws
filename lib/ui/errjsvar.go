package ui

import (
	"errors"

	"github.com/linkdata/jaws"
)

// ErrIllegalJsVarName reports an invalid or reserved browser name.
var ErrIllegalJsVarName errIllegalJsVarName

type errIllegalJsVarName string

func (e errIllegalJsVarName) Error() string {
	if e != "" {
		return "illegal jsvar name: " + string(e)
	}
	return "illegal jsvar name"
}

func (errIllegalJsVarName) Is(target error) bool { return target == ErrIllegalJsVarName }

// ErrIllegalJsVarPath reports an invalid or reserved dotted path.
var ErrIllegalJsVarPath = errors.New("jsvar: invalid path")

// ErrJsVarNameConflict reports different stores using one browser name in a Request.
var ErrJsVarNameConflict = errors.New("jsvar: browser name bound to another store")

// ErrJsVarBindingUsed reports a binding rendered more than once.
var ErrJsVarBindingUsed = errors.New("jsvar: binding already rendered")

// ErrJsVarReadOnly reports a browser proposal to a store without ClientCheck.
var ErrJsVarReadOnly = errors.New("jsvar: browser writes denied")

// ErrJsVarTooLarge reports a failed JSON size check and cancels the source Request.
var ErrJsVarTooLarge = errors.New("jsvar: JSON size check failed")

type errJsVarClientWrite struct{ cause error }

func (e errJsVarClientWrite) Error() string { return e.cause.Error() }
func (e errJsVarClientWrite) Is(target error) bool {
	// Event dispatch treats ErrEventUnhandled as fallthrough. A rejected proposal
	// is always handled, even when an application check returns that sentinel.
	return target != jaws.ErrEventUnhandled && errors.Is(e.cause, target)
}

// JawsClientAlert returns a generic browser message for a rejected proposal.
func (errJsVarClientWrite) JawsClientAlert() string { return "invalid JsVar update" }
