package jawsbool

import (
	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
)

type (
	// Element is an alias for core.Element.
	Element = core.Element
	// UI is an alias for core.UI.
	UI = core.UI
	// Container is an alias for core.Container.
	Container = core.Container
	// Setter is an alias for jawsbind.Setter.
	Setter[T comparable] = jawsbind.Setter[T]
)

var (
	// ErrValueUnchanged indicates a setter call changed nothing.
	ErrValueUnchanged = core.ErrValueUnchanged
)
