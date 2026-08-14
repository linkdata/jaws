package named

import (
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// SelectHandler renders select options and stores the selection as a string.
//
// Rendered option values must be non-empty. A string that matches no rendered
// option value represents no selection. [BoolArray] is the standard
// implementation and returns an empty string when no [Bool] is checked.
type SelectHandler interface {
	jaws.Container
	bind.Setter[string]
}
