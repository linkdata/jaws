package named

import (
	"html/template"
	"strings"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
)

// BoolArray stores the data required to support HTML select elements
// and sets of HTML radio buttons. It is safe to use from multiple goroutines
// concurrently.
type BoolArray struct {
	multi bool             // allow multiple NamedBools to be true
	mu    deadlock.RWMutex // protects following
	data  []*Bool
}

var _ SelectHandler = (*BoolArray)(nil)

// NewBoolArray returns an empty [BoolArray].
//
// If multi is false, setting one value clears other names in the array. If
// multi is true, multiple values may be checked at the same time.
func NewBoolArray(multi bool) *BoolArray {
	return &BoolArray{multi: multi}
}

// ReadLocked calls fn with the [BoolArray] locked for reading.
func (nba *BoolArray) ReadLocked(fn func(nbl []*Bool)) {
	nba.mu.RLock()
	defer nba.mu.RUnlock()
	fn(nba.data)
}

// WriteLocked calls fn with the [BoolArray] locked for writing and replaces
// the internal []*Bool slice with the return value.
func (nba *BoolArray) WriteLocked(fn func(nbl []*Bool) []*Bool) {
	nba.mu.Lock()
	defer nba.mu.Unlock()
	nba.data = fn(nba.data)
}

// JawsContains returns the option widgets for a select backed by nba.
func (nba *BoolArray) JawsContains(elem *jaws.Element) (contents []jaws.UI) {
	nba.mu.RLock()
	for _, nb := range nba.data {
		contents = append(contents, namedBoolOption{nb})
	}
	nba.mu.RUnlock()
	return
}

// Add adds a [Bool] with the given name and trusted HTML text.
// Returns itself.
//
// Note that while it is legal to have multiple [Bool] values with the same
// name because HTML allows it, it is usually not a good idea.
func (nba *BoolArray) Add(name string, text template.HTML) *BoolArray {
	nba.mu.Lock()
	nba.data = append(nba.data, NewBool(nba, name, text, false))
	nba.mu.Unlock()
	return nba
}

// Set sets the checked state for [Bool] values with the given name.
//
// If the given name does not match any values in single-select
// mode, everything will be deselected.
func (nba *BoolArray) Set(name string, state bool) (changed bool) {
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

// deselectOthersLocked clears all Bools whose name differs from
// the given name when the array is in single-select mode and state is true.
func (nba *BoolArray) deselectOthersLocked(name string, state bool) (changed bool) {
	if state && !nba.multi {
		for _, nb := range nba.data {
			if nb.Name() != name {
				changed = nb.Set(false) || changed
			}
		}
	}
	return
}

// Get returns the name of the first [Bool] in the group that
// has its checked value set to true. Returns an empty string
// if none are true.
//
// In case you can have more than one selected or you need to
// distinguish between a blank name and the fact that none are
// set to true, use [BoolArray.ReadLocked] to inspect the data directly.
func (nba *BoolArray) Get() (name string) {
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

// Count returns the number of [Bool] values in the set that have the given name.
func (nba *BoolArray) Count(name string) (n int) {
	nba.mu.RLock()
	defer nba.mu.RUnlock()
	for _, nb := range nba.data {
		if nb.Name() == name {
			n++
		}
	}
	return
}

// IsChecked returns true if any [Bool] in the set with the
// given name are Checked. Returns false if the name is not found.
func (nba *BoolArray) IsChecked(name string) (state bool) {
	nba.mu.RLock()
	defer nba.mu.RUnlock()
	for _, nb := range nba.data {
		if nb.Name() == name && nb.Checked() {
			return true
		}
	}
	return false
}

// String returns a string representation of the [BoolArray] suitable for debugging.
func (nba *BoolArray) String() string {
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

// JawsGet returns the currently selected name.
func (nba *BoolArray) JawsGet(elem *jaws.Element) string {
	return nba.Get()
}

// JawsSet selects name and dirties nba if the selection changed.
func (nba *BoolArray) JawsSet(elem *jaws.Element, name string) (err error) {
	if nba.Set(name, true) {
		elem.Dirty(nba)
	} else {
		err = jaws.ErrValueUnchanged
	}
	return
}
