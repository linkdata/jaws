package core

import (
	"html/template"
	"strings"

	"github.com/linkdata/deadlock"
)

// NamedBoolArray stores the data required to support HTML 'select' elements
// and sets of HTML radio buttons. It it safe to use from multiple goroutines
// concurrently.
type NamedBoolArray struct {
	Multi bool             // allow multiple NamedBools to be true
	mu    deadlock.RWMutex // protects following
	data  []*NamedBool
}

var _ SelectHandler = (*NamedBoolArray)(nil)

func NewNamedBoolArray() *NamedBoolArray {
	return &NamedBoolArray{}
}

// ReadLocked calls the given function with the NamedBoolArray locked for reading.
func (nba *NamedBoolArray) ReadLocked(fn func(nbl []*NamedBool)) {
	nba.mu.RLock()
	defer nba.mu.RUnlock()
	fn(nba.data)
}

// WriteLocked calls the given function with the NamedBoolArray locked for writing and
// replaces the internal []*NamedBool slice with the return value.
func (nba *NamedBoolArray) WriteLocked(fn func(nbl []*NamedBool) []*NamedBool) {
	nba.mu.Lock()
	defer nba.mu.Unlock()
	nba.data = fn(nba.data)
}

func (nba *NamedBoolArray) JawsContains(e *Element) (contents []UI) {
	nba.mu.RLock()
	for _, nb := range nba.data {
		contents = append(contents, namedBoolOption{nb})
	}
	nba.mu.RUnlock()
	return
}

// Add adds a NamedBool with the given name and the given text.
// Returns itself.
//
// Note that while it's legal to have multiple NamedBool with the same name
// since it's allowed in HTML, it's probably not a good idea.
func (nba *NamedBoolArray) Add(name string, text template.HTML) *NamedBoolArray {
	nba.mu.Lock()
	nba.data = append(nba.data, NewNamedBool(nba, name, text, false))
	nba.mu.Unlock()
	return nba
}

// Set sets the Checked state for the NamedBool(s) with the given name.
func (nba *NamedBoolArray) Set(name string, state bool) (changed bool) {
	nba.mu.Lock()
	defer nba.mu.Unlock()
	for _, nb := range nba.data {
		if nb.Name() == name {
			changed = nb.Set(state) || changed
		}
	}
	changed = nba.deselectOthersLocked(name, state) || changed
	return
}

// deselectOthersLocked clears all NamedBools whose name differs from
// the given name when the array is in single-select mode and state is true.
func (nba *NamedBoolArray) deselectOthersLocked(name string, state bool) (changed bool) {
	if state && !nba.Multi {
		for _, nb := range nba.data {
			if nb.Name() != name {
				changed = nb.Set(false) || changed
			}
		}
	}
	return
}

// Get returns the name of first NamedBool in the group that
// has it's Checked value set to true. Returns an empty string
// if none are true.
//
// In case you can have more than one selected or you need to
// distinguish between a blank name and the fact that none are
// set to true, use ReadLocked() to inspect the data directly.
func (nba *NamedBoolArray) Get() (name string) {
	nba.mu.RLock()
	for _, nb := range nba.data {
		if nb.Checked() {
			name = nb.Name()
			break
		}
	}
	nba.mu.RUnlock()
	return
}

func (nba *NamedBoolArray) isCheckedLocked(name string) bool {
	for _, nb := range nba.data {
		if nb.Checked() && nb.Name() == name {
			return true
		}
	}
	return false
}

// IsChecked returns true if any of the NamedBool in the set that have the
// given name are Checked. Returns false if the name is not found.
func (nba *NamedBoolArray) IsChecked(name string) (state bool) {
	nba.mu.RLock()
	state = nba.isCheckedLocked(name)
	nba.mu.RUnlock()
	return
}

// String returns a string representation of the NamedBoolArray suitable for debugging.
func (nba *NamedBoolArray) String() string {
	var sb strings.Builder
	sb.WriteString("&NamedBoolArray{[")
	nba.mu.RLock()
	for i, nb := range nba.data {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(nb.String())
	}
	nba.mu.RUnlock()
	sb.WriteString("]}")
	return sb.String()
}

func (nba *NamedBoolArray) JawsGet(e *Element) string {
	return nba.Get()
}

func (nba *NamedBoolArray) JawsSet(e *Element, name string) (err error) {
	if nba.Set(name, true) {
		e.Dirty(nba)
	} else {
		err = ErrValueUnchanged
	}
	return
}
