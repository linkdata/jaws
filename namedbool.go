package jaws

import (
	"fmt"
	"html/template"

	"github.com/linkdata/deadlock"
)

// NamedBool stores a named boolen value with a HTML representation.
type NamedBool struct {
	nba     *NamedBoolArray  // (read-only) NamedBoolArray that this is part of (may be nil)
	mu      deadlock.RWMutex // protects following
	name    string           // name within the named bool set
	html    template.HTML    // HTML code used in select lists or labels
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
	nb.mu.RLock()
	s = nb.name
	nb.mu.RUnlock()
	return
}

func (nb *NamedBool) Html() (h template.HTML) {
	nb.mu.RLock()
	h = nb.html
	nb.mu.RUnlock()
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

var _ ValueProxy = (*NamedBool)(nil)
var _ ValueProxy = namedBoolHtmlValueProxy{}

type namedBoolHtmlValueProxy struct {
	nb *NamedBool
}

func (hvp namedBoolHtmlValueProxy) JawsGet(e *Element) interface{} {
	hvp.nb.mu.RLock()
	html := hvp.nb.html
	hvp.nb.mu.RUnlock()
	return html
}

func (hvp namedBoolHtmlValueProxy) JawsSet(e *Element, value interface{}) (changed bool) {
	if html, ok := value.(template.HTML); ok {
		hvp.nb.mu.Lock()
		if hvp.nb.html != html {
			hvp.nb.html = html
			changed = true
		}
		hvp.nb.mu.Unlock()
	}
	return
}

func (nb *NamedBool) HtmlValueProxy() ValueProxy {
	return namedBoolHtmlValueProxy{nb}
}

func (nb *NamedBool) JawsGet(e *Element) interface{} {
	nb.mu.RLock()
	checked := nb.checked
	nb.mu.RUnlock()
	return checked
}

func (nb *NamedBool) JawsSet(e *Element, value interface{}) (changed bool) {
	if checked, ok := value.(bool); ok {
		nb.mu.Lock()
		if nb.checked != checked {
			nb.checked = checked
			changed = true
		}
		nb.mu.Unlock()
		if changed {
			e.Dirty()
			if checked && nb.nba != nil {
				nb.nba.Set(nb.name, true)
			}
		}
		return
	}
	panic("NamedBool.JawsSet(): not bool")
}
