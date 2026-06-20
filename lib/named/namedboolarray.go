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
	multi bool             // allow multiple Bools to be true
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
//
// fn must not call other [BoolArray] methods or [Bool.JawsSet]: those re-acquire
// the same non-reentrant nba mutex and deadlock. It may operate on the provided
// slice and call the *[Bool] methods that take only the Bool's own mutex — the
// reads [Bool.Name], [Bool.HTML], [Bool.JawsGet], [Bool.Checked], [Bool.String]
// (and similar) and the write [Bool.Set]. [Bool.Set] mutates the Bool under the
// Bool's own mutex, which the nba read lock held here does not serialize, so
// concurrent callers must coordinate any such writes themselves.
func (nba *BoolArray) ReadLocked(fn func(nbl []*Bool)) {
	nba.mu.RLock()
	defer nba.mu.RUnlock()
	fn(nba.data)
}

// WriteLocked calls fn with the [BoolArray] locked for writing and replaces
// the internal []*Bool slice with the return value.
//
// As with [BoolArray.ReadLocked], fn must not call other [BoolArray] methods or
// [Bool.JawsSet] (they re-acquire the same non-reentrant nba mutex and deadlock);
// operate only on the provided slice and the *[Bool] methods that take just the
// Bool's own mutex (see [BoolArray.ReadLocked]). The exclusive lock held here
// serializes [Bool.Set] across concurrent WriteLocked calls.
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
// The html argument is rendered as trusted HTML and is not escaped; pre-escape it
// (e.g. template.HTML(template.HTMLEscapeString(s))) when it is derived from
// untrusted user input. See [NewBool].
//
// Note that while it is legal to have multiple [Bool] values with the same
// name because HTML allows it, it is usually not a good idea.
func (nba *BoolArray) Add(name string, html template.HTML) *BoolArray {
	nba.mu.Lock()
	nba.data = append(nba.data, NewBool(nba, name, html, false))
	nba.mu.Unlock()
	return nba
}

// Set sets the checked state for [Bool] values with the given name.
//
// Matching is by name, so values are addressed as logical options rather than
// individually: every [Bool] sharing the name is set together, and in
// single-select mode the at-most-one-checked invariant holds per distinct name
// (selecting a name deselects all values with a different name, but leaves
// same-named siblings checked). If the given name matches no values in
// single-select mode, everything will be deselected.
func (nba *BoolArray) Set(name string, state bool) (changed bool) {
	nba.mu.Lock()
	defer nba.mu.Unlock()
	return len(nba.setChangedLocked(name, state)) > 0
}

// setChangedLocked sets the [Bool] values with the given name to state, applies
// single-select deselection, and returns every [Bool] whose state changed. The
// BoolArray must be locked for writing.
func (nba *BoolArray) setChangedLocked(name string, state bool) (changed []*Bool) {
	for _, nb := range nba.data {
		if nb.Name() == name {
			if nb.Set(state) {
				changed = append(changed, nb)
			}
		}
	}
	changed = append(changed, nba.deselectOthersLocked(name, state)...)
	return
}

// deselectOthersLocked clears all Bools whose name differs from
// the given name when the array is in single-select mode and state is true.
func (nba *BoolArray) deselectOthersLocked(name string, state bool) (changed []*Bool) {
	if state && !nba.multi {
		for _, nb := range nba.data {
			if nb.Name() != name {
				if nb.Set(false) {
					changed = append(changed, nb)
				}
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
// given name is checked. Returns false if the name is not found.
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
	sb.WriteString("&BoolArray{[")
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

// JawsSet selects name and dirties the changed [Bool] values and nba itself.
//
// This mirrors [Bool.JawsSet]: every Bool whose checked state changes is dirtied
// in addition to the array tag, so consumers that bind individual Bools (such as
// radio buttons) update, not only the cascading [github.com/linkdata/jaws/lib/ui.Select] widget that re-renders
// from the array tag.
//
// In single-select mode a name matching no [Bool] still succeeds (returns nil) by
// deselecting the current selection, as documented for [BoolArray.Set], leaving
// the selected name empty. A nil return therefore means "the selection changed",
// not "name is now selected".
func (nba *BoolArray) JawsSet(elem *jaws.Element, name string) (err error) {
	nba.mu.Lock()
	changed := nba.setChangedLocked(name, true)
	nba.mu.Unlock()
	if len(changed) == 0 {
		return jaws.ErrValueUnchanged
	}
	for _, nb := range changed {
		elem.Dirty(nb)
	}
	elem.Dirty(nba)
	return
}
