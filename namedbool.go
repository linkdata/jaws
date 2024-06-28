package jaws

import (
	"fmt"
	"html/template"

	"github.com/linkdata/deadlock"
)

// NamedBool stores a named boolen value with a HTML representation.
type NamedBool struct {
	nba     *NamedBoolArray  // (read-only) NamedBoolArray that this is part of (may be nil)
	name    string           // (read-only) name within the named bool set
	html    template.HTML    // (read-only) HTML code used in select lists or labels
	mu      deadlock.RWMutex // protects following
	checked bool             // it's state
}

func NewNamedBool(nba *NamedBoolArray, name string, html template.HTML, checked bool) *NamedBool {
	return &NamedBool{
		nba:     nba,
		name:    name,
		html:    html,
		checked: checked,
	}
}

func (nb *NamedBool) Array() *NamedBoolArray {
	return nb.nba
}

func (nb *NamedBool) Name() (s string) {
	s = nb.name
	return
}

func (nb *NamedBool) Html() (h template.HTML) {
	h = nb.html
	return
}

func (nb *NamedBool) JawsGetString(*Element) (name string) {
	return nb.Name()
}

func (nb *NamedBool) JawsGetHtml(*Element) (h template.HTML) {
	return nb.Html()
}

func (nb *NamedBool) JawsGetBool(*Element) (v bool) {
	nb.mu.RLock()
	v = nb.checked
	nb.mu.RUnlock()
	return
}

func (nb *NamedBool) JawsSetBool(e *Element, checked bool) (err error) {
	var nba *NamedBoolArray
	nb.mu.Lock()
	if nb.checked != checked {
		nb.checked = checked
		nba = nb.nba
	} else {
		err = ErrValueUnchanged
	}
	nb.mu.Unlock()
	if err == nil {
		e.Dirty(nb)
		if nba != nil {
			e.Dirty(nba)
			nb.nba.Set(nb.name, checked)
		}
	}
	return
}

func (nb *NamedBool) Checked() (checked bool) {
	nb.mu.RLock()
	checked = nb.checked
	nb.mu.RUnlock()
	return
}

func (nb *NamedBool) Set(checked bool) (changed bool) {
	nb.mu.Lock()
	if nb.checked != checked {
		nb.checked = checked
		changed = true
	}
	nb.mu.Unlock()
	return
}

// String returns a string representation of the NamedBool suitable for debugging.
func (nb *NamedBool) String() string {
	return fmt.Sprintf("&{%q,%q,%v}", nb.Name(), nb.Html(), nb.Checked())
}
