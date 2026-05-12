package named

import (
	"fmt"
	"html/template"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
)

// Bool stores a named boolean value with an HTML representation.
//
// Bool values are safe for concurrent use.
type Bool struct {
	nba     *BoolArray       // (read-only) NamedBoolArray that this is part of (may be nil)
	name    string           // (read-only) name within the named bool set
	html    template.HTML    // (read-only) HTML code used in select lists or labels
	mu      deadlock.RWMutex // protects following
	checked bool             // its state
}

// NewBool returns a [Bool] with the given name, HTML and checked state.
//
// If nba is non-nil, changing the value through [Bool.JawsSet] may dirty the
// containing [BoolArray] and deselect sibling values in single-select mode.
func NewBool(nba *BoolArray, name string, html template.HTML, checked bool) *Bool {
	return &Bool{
		nba:     nba,
		name:    name,
		html:    html,
		checked: checked,
	}
}

// Array returns the [BoolArray] that owns nb, or nil.
func (nb *Bool) Array() *BoolArray {
	return nb.nba
}

// Name returns the form value name for nb.
func (nb *Bool) Name() (s string) {
	s = nb.name
	return
}

// HTML returns the trusted HTML label for nb.
func (nb *Bool) HTML() (h template.HTML) {
	h = nb.html
	return
}

// JawsGetHTML returns the trusted HTML label for nb.
func (nb *Bool) JawsGetHTML(*jaws.Element) (h template.HTML) {
	return nb.HTML()
}

// JawsGet returns whether nb is checked.
func (nb *Bool) JawsGet(*jaws.Element) (v bool) {
	nb.mu.RLock()
	v = nb.checked
	nb.mu.RUnlock()
	return
}

// JawsSet sets the checked state and dirties the affected element tags.
func (nb *Bool) JawsSet(e *jaws.Element, checked bool) (err error) {
	err = jaws.ErrValueUnchanged
	nba := nb.nba
	if nba != nil {
		nba.mu.Lock()
		defer nba.mu.Unlock()
	}
	nb.mu.Lock()
	if nb.checked != checked {
		nb.checked = checked
		err = nil
	}
	nb.mu.Unlock()
	if err == nil {
		e.Dirty(nb)
		if nba != nil {
			nba.deselectOthersLocked(nb.name, checked)
			e.Dirty(nba)
		}
	}
	return
}

// Checked reports whether nb is checked.
func (nb *Bool) Checked() (checked bool) {
	nb.mu.RLock()
	checked = nb.checked
	nb.mu.RUnlock()
	return
}

// Set changes the checked state and reports whether it changed.
func (nb *Bool) Set(checked bool) (changed bool) {
	nb.mu.Lock()
	if nb.checked != checked {
		nb.checked = checked
		changed = true
	}
	nb.mu.Unlock()
	return
}

// String returns a string representation of the [Bool] suitable for debugging.
func (nb *Bool) String() string {
	return fmt.Sprintf("&{%q,%q,%v}", nb.Name(), nb.HTML(), nb.Checked())
}
