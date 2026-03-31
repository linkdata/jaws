package named

import (
	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type (
	// Element is an alias for core.Element.
	Element = core.Element
	// UI is an alias for core.UI.
	UI = core.UI
	// Container is an alias for core.Container.
	Container = core.Container
	// Setter is an alias for bind.Setter.
	Setter[T comparable] = bind.Setter[T]
)

var (
	// ErrValueUnchanged indicates a setter call changed nothing.
	ErrValueUnchanged = core.ErrValueUnchanged
)
