package named

import (
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// SelectHandler is implemented by values that can both render options and
// store a selected option name. [BoolArray] is the standard implementation.
type SelectHandler interface {
	jaws.Container
	bind.Setter[string]
}
