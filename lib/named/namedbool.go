package named

import (
	"fmt"
	"html/template"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
)

// Bool stores a named boolen value with a HTML representation.
type Bool struct {
	nba     *BoolArray       // (read-only) NamedBoolArray that this is part of (may be nil)
	name    string           // (read-only) name within the named bool set
	html    template.HTML    // (read-only) HTML code used in select lists or labels
	mu      deadlock.RWMutex // protects following
	checked bool             // it's state
}

func NewBool(nba *BoolArray, name string, html template.HTML, checked bool) *Bool {
	return &Bool{
		nba:     nba,
		name:    name,
		html:    html,
		checked: checked,
	}
}

func (nb *Bool) Array() *BoolArray {
	return nb.nba
}

func (nb *Bool) Name() (s string) {
	s = nb.name
	return
}

func (nb *Bool) HTML() (h template.HTML) {
	h = nb.html
	return
}

func (nb *Bool) JawsGetHTML(*jaws.Element) (h template.HTML) {
	return nb.HTML()
}

func (nb *Bool) JawsGet(*jaws.Element) (v bool) {
	nb.mu.RLock()
	v = nb.checked
	nb.mu.RUnlock()
	return
}

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

func (nb *Bool) Checked() (checked bool) {
	nb.mu.RLock()
	checked = nb.checked
	nb.mu.RUnlock()
	return
}

func (nb *Bool) Set(checked bool) (changed bool) {
	nb.mu.Lock()
	if nb.checked != checked {
		nb.checked = checked
		changed = true
	}
	nb.mu.Unlock()
	return
}

// String returns a string representation of the NamedBool suitable for debugging.
func (nb *Bool) String() string {
	return fmt.Sprintf("&{%q,%q,%v}", nb.Name(), nb.HTML(), nb.Checked())
}
