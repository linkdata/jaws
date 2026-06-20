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
	nba     *BoolArray       // (read-only) BoolArray that this is part of (may be nil)
	name    string           // (read-only) name within the named bool set
	html    template.HTML    // (read-only) HTML code used in select lists or labels
	mu      deadlock.RWMutex // protects following
	checked bool             // its state
}

// NewBool returns a [Bool] with the given name, HTML and checked state.
//
// The html argument is rendered as trusted HTML (it is the label shown in select
// lists and checkboxes) and is not escaped. When it is derived from untrusted
// user input it must be pre-escaped, e.g. template.HTML(template.HTMLEscapeString(s)).
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
func (nb *Bool) Name() string {
	return nb.name
}

// HTML returns the trusted HTML label for nb.
func (nb *Bool) HTML() template.HTML {
	return nb.html
}

// JawsGetHTML returns the trusted HTML label for nb.
func (nb *Bool) JawsGetHTML(elem *jaws.Element) (h template.HTML) {
	return nb.HTML()
}

// JawsGet returns whether nb is checked.
func (nb *Bool) JawsGet(elem *jaws.Element) (yes bool) {
	nb.mu.RLock()
	yes = nb.checked
	nb.mu.RUnlock()
	return
}

// JawsSet sets the checked state and dirties the affected element tags.
//
// It returns [jaws.ErrValueUnchanged] if checked already matched the current
// state. Affected tags are dirtied after the value locks have been released,
// matching [BoolArray.JawsSet].
func (nb *Bool) JawsSet(elem *jaws.Element, checked bool) (err error) {
	nba := nb.nba
	// Lock ordering invariant: when both locks are needed, the owning BoolArray's
	// mutex is always acquired before the Bool's mutex (as done here and by the
	// BoolArray methods, which lock nba.mu then call into Bool). Any new method
	// must preserve this array-before-bool order to avoid deadlocks. Dirtying is
	// deferred until after the value locks are released to avoid a
	// value-lock-to-Jaws-lock inversion.
	if nba != nil {
		nba.mu.Lock()
	}
	var changed []*Bool
	nb.mu.Lock()
	if nb.checked != checked {
		nb.checked = checked
		changed = append(changed, nb)
	}
	nb.mu.Unlock()
	if nba != nil {
		changed = append(changed, nba.deselectOthersLocked(nb.name, checked)...)
		nba.mu.Unlock()
	}
	if len(changed) == 0 {
		err = jaws.ErrValueUnchanged
		return
	}
	for _, nb := range changed {
		elem.Dirty(nb)
	}
	if nba != nil {
		elem.Dirty(nba)
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
//
// Unlike [Bool.JawsSet], Set does not dirty any elements and does not deselect
// siblings in single-select mode; it only changes this value. Single-select
// consistency (at most one checked value) is an invariant maintained by going
// through JawsSet, so prefer JawsSet for widget-driven updates.
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
	return fmt.Sprintf("&Bool{%q,%q,%v}", nb.Name(), nb.HTML(), nb.Checked())
}
