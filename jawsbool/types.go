package jawsbool

import (
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/bind"
)

type (
	// Element is an alias for jaws.Element.
	Element = jaws.Element
	// UI is an alias for jaws.UI.
	UI = jaws.UI
	// Container is an alias for jaws.Container.
	Container = jaws.Container
	// Setter is an alias for jawsbind.Setter.
	Setter[T comparable] = bind.Setter[T]
)

var (
	// ErrValueUnchanged indicates a setter call changed nothing.
	ErrValueUnchanged = jaws.ErrValueUnchanged
)
