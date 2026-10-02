package named

import (
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// SelectHandler renders single-select options and stores one selected value.
//
// Rendered option values must be non-empty. A string that matches no rendered
// option value represents no selection. [BoolArray] is the standard
// implementation when created with NewBoolArray(false) or its zero value. It
// returns an empty string when no [Bool] is checked.
type SelectHandler interface {
	jaws.Container
	bind.Setter[string]
}
