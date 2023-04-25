package jaws

import (
	"fmt"

	"github.com/linkdata/deadlock"
)

type NamedBool struct {
	Name    string // name within the named bool set
	Text    string // HTML text of the boolean
	Checked bool   // it's state
}

// String returns a string representation of the NamedBool suitable for debugging.
func (nb *NamedBool) String() string {
	return fmt.Sprintf("&{%q,%q,%v}", nb.Name, nb.Text, nb.Checked)
}

// NamedBoolArray stores the data required to support HTML 'select' elements
// and sets of HTML radio buttons. It it safe to use from multiple goroutines
// concurrently.
type NamedBoolArray struct {
	ID   string           // (read-only) JaWS ID of the array
	mu   deadlock.RWMutex // protects following
	data []*NamedBool
}

// NewNamedBoolArray creates a new object to track a related set of named booleans.
// The JaWS ID string 'jid' is used as the ID for <select> elements and the
// value for the 'name' attribute for radio buttons.
func NewNamedBoolArray(jid string) *NamedBoolArray {
	return &NamedBoolArray{ID: jid}
}

// ReadLocked calls the given function with the NamedBoolArray locked for reading.
func (nba *NamedBoolArray) ReadLocked(fn func(nba []*NamedBool)) {
	nba.mu.RLock()
	defer nba.mu.RUnlock()
	fn(nba.data)
}

// WriteLocked calls the given function with the NamedBoolArray locked for writing and
// replaces the internal []*NamedBool slice with the return value.
func (nba *NamedBoolArray) WriteLocked(fn func(nba []*NamedBool) []*NamedBool) {
	nba.mu.Lock()
	defer nba.mu.Unlock()
	nba.data = fn(nba.data)
}

// Add adds a NamedBool with the given name and the given text.
//
// Note that while it's legal to have multiple NamedBool with the same name
// since it's allowed in HTML, it's probably not a good idea.
func (nba *NamedBoolArray) Add(name, text string) {
	nba.mu.Lock()
	nba.data = append(nba.data, &NamedBool{Name: name, Text: text})
	nba.mu.Unlock()
}

// SetSelect sets the Checked state for the NamedBool(s) with the given name.
func (nba *NamedBoolArray) SetSelect(name string, state bool) {
	nba.mu.Lock()
	for _, nb := range nba.data {
		if nb.Name == name {
			nb.Checked = state
		}
	}
	nba.mu.Unlock()
}

// SetRadio sets the Checked state for the NamedBool(s) with the
// given name to true and all others to false.
func (nba *NamedBoolArray) SetRadio(name string) {
	nba.mu.Lock()
	for _, nb := range nba.data {
		nb.Checked = (nb.Name == name)
	}
	nba.mu.Unlock()
}

// Selected returns the name of first checked NamedBool in the set.
// Returns an empty string if none are checked.
//
// In case you can have more than one selected or you need to
// distinguish between a blank name and the fact that none are
// checked, use ReadLocked() to inspect the array directly.
func (nba *NamedBoolArray) Selected() (name string) {
	nba.mu.RLock()
	for _, nb := range nba.data {
		if nb.Checked {
			name = nb.Name
			break
		}
	}
	nba.mu.RUnlock()
	return
}

// String returns a string representation of the NamedBoolArray suitable for debugging.
func (nba *NamedBoolArray) String() string {
	nba.mu.RLock()
	b := make([]byte, 0, 17+len(nba.data)*16)
	b = append(b, "&NamedBoolArray{"...)
	for i, v := range nba.data {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(fmt.Sprint(v))...)
	}
	b = append(b, '}')
	nba.mu.RUnlock()
	return string(b)
}
