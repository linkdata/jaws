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

// MultiSelectHandler renders multi-select options and stores selected values.
//
// Rendered option values must be distinct and non-empty. JawsGetValues returns
// the selected values; JawsSetValues replaces the entire selection, with nil or
// an empty slice clearing it. [BoolArray] created with NewBoolArray(true) is the
// standard implementation.
type MultiSelectHandler interface {
	jaws.Container
	JawsGetValues(*jaws.Element) []string
	JawsSetValues(*jaws.Element, []string) error
}
