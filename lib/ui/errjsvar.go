package ui

import (
	"errors"

	"github.com/linkdata/jaws"
)

// ErrIllegalJsVarName reports an invalid or reserved browser name.
var ErrIllegalJsVarName = errors.New("illegal jsvar name")

// ErrIllegalJsVarPath reports an invalid path or unsupported complex-shape
// proposal.
var ErrIllegalJsVarPath = errors.New("jsvar: invalid path")

// ErrJsVarNameConflict reports duplicate or overlapping browser names in a Request.
var ErrJsVarNameConflict = errors.New("jsvar: browser name already bound")

// ErrJsVarBindingUsed reports a binding rendered more than once.
var ErrJsVarBindingUsed = errors.New("jsvar: binding already rendered")

// ErrJsVarBindingWrongJaws reports a binding rendered on another [jaws.Jaws].
var ErrJsVarBindingWrongJaws = errors.New("jsvar: binding rendered on a different Jaws instance")

// ErrJsVarReadOnly reports a browser proposal to a store without ClientCheck.
var ErrJsVarReadOnly = errors.New("jsvar: browser writes denied")

// ErrJsVarTooLarge reports a failed JSON size check.
//
// A browser proposal rejected with this error cancels its source Request.
var ErrJsVarTooLarge = errors.New("jsvar: JSON size check failed")

type errJsVarClientWrite struct{ cause error }

func (e errJsVarClientWrite) Error() string { return e.cause.Error() }
func (e errJsVarClientWrite) Is(target error) bool {
	// Event dispatch treats ErrEventUnhandled as fallthrough. A rejected proposal
	// is always handled, even when an application check returns that sentinel.
	return target == jaws.ErrEventLogOnly || (target != jaws.ErrEventUnhandled && errors.Is(e.cause, target))
}
