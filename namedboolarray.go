package jaws

import (
	"fmt"

	"github.com/linkdata/deadlock"
)

type NamedBool struct {
	Name    string // name unique within the named bool set
	Text    string // HTML text of the boolean
	Checked bool   // it's state
}

func (nb *NamedBool) String() string {
	return fmt.Sprintf("&{%q,%q,%v}", nb.Name, nb.Text, nb.Checked)
}

// NamedBools stores the data required to support HTML 'select' elements
// and sets of HTML radio buttons. It handles locking of it's data.
type NamedBools struct {
	ID   string           // (read-only) JaWS ID of the entire set
	mu   deadlock.RWMutex // protects following
	data []*NamedBool
}

// NewNamedBools creates a new object to track a related set of named booleans.
// The JaWS ID string 'jid' is used as the ID for <select> elements and the
// value for the 'name' attribute for radio buttons.
func NewNamedBools(jid string) *NamedBools {
	return &NamedBools{ID: jid}
}

// ReadLocked calls the given function with the NamedBools locked for reading.
func (nbs *NamedBools) ReadLocked(fn func(nba []*NamedBool)) {
	nbs.mu.RLock()
	defer nbs.mu.RUnlock()
	fn(nbs.data)
}

// WriteLocked calls the given function with the NamedBools locked for writing and
// replaces the internal []*NamedBool slice with the return value.
func (nbs *NamedBools) WriteLocked(fn func(nba []*NamedBool) []*NamedBool) {
	nbs.mu.Lock()
	defer nbs.mu.Unlock()
	nbs.data = fn(nbs.data)
}

// Add ensures that a NamedBool with the given name exists and has the given text.
func (nbs *NamedBools) Add(name, text string) {
	nbs.mu.Lock()
	defer nbs.mu.Unlock()
	for _, nb := range nbs.data {
		if nb.Name == name {
			nb.Text = text
			return
		}
	}
	nbs.data = append(nbs.data, &NamedBool{Name: name, Text: text})
}

// SetSelect sets the Checked state for the NamedBool with the given name.
func (nbs *NamedBools) SetSelect(name string, state bool) {
	nbs.mu.Lock()
	for _, nb := range nbs.data {
		if nb.Name == name {
			nb.Checked = state
			break
		}
	}
	nbs.mu.Unlock()
}

// SetRadio sets the Checked state for the NamedBool with the
// given name to true and all others to false.
func (nbs *NamedBools) SetRadio(name string) {
	nbs.mu.Lock()
	for _, nb := range nbs.data {
		nb.Checked = (nb.Name == name)
	}
	nbs.mu.Unlock()
}

// GetRadio returns the name of first checked NamedBool in the set.
// Returns an empty string if none are checked.
func (nbs *NamedBools) GetRadio() (name string) {
	nbs.mu.RLock()
	for _, nb := range nbs.data {
		if nb.Checked {
			name = nb.Name
			break
		}
	}
	nbs.mu.RUnlock()
	return
}

func (nbs *NamedBools) String() string {
	nbs.mu.RLock()
	b := make([]byte, 0, 17+len(nbs.data)*16)
	b = append(b, "&NamedBools{"...)
	for i, v := range nbs.data {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(fmt.Sprint(v))...)
	}
	b = append(b, '}')
	nbs.mu.RUnlock()
	return string(b)
}
