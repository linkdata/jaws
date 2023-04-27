package jaws

import (
	"strconv"
	"strings"

	"github.com/linkdata/deadlock"
)

// NamedBoolArray stores the data required to support HTML 'select' elements
// and sets of HTML radio buttons. It it safe to use from multiple goroutines
// concurrently.
type NamedBoolArray struct {
	Jid    string           // (read-only) JaWS ID of the array
	prefix string           // Jid+"/"
	mu     deadlock.RWMutex // protects following
	data   []*NamedBool
}

// NewNamedBoolArray creates a new object to track a related set of named booleans.
//
// The JaWS ID string 'jid' is used as the ID for <select> elements and the
// value for the 'name' attribute for radio buttons. If left empty, MakeID() will
// be used to assign a unique ID.
func NewNamedBoolArray(jid string) *NamedBoolArray {
	if jid == "" {
		jid = MakeID()
	}
	return &NamedBoolArray{Jid: jid, prefix: jid + "/"}
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

// Add adds a NamedBool with the given name and the given text.
//
// Note that while it's legal to have multiple NamedBool with the same name
// since it's allowed in HTML, it's probably not a good idea.
func (nba *NamedBoolArray) Add(name, text string) {
	nb := &NamedBool{Name: name, Html: text}
	nba.mu.Lock()
	nba.data = append(nba.data, nb)
	nba.mu.Unlock()
}

// Set sets the Checked state for the NamedBool(s) with the given name or Jid.
func (nba *NamedBoolArray) Set(name string, state bool) {
	name = strings.TrimPrefix(name, nba.prefix)
	nba.mu.Lock()
	for _, nb := range nba.data {
		if nb.Name == name {
			nb.Checked = state
		}
	}
	nba.mu.Unlock()
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
		if nb.Checked {
			name = nb.Name
			break
		}
	}
	nba.mu.RUnlock()
	return
}

func (nba *NamedBoolArray) JidOf(name string) string {
	return nba.prefix + name
}

// SetOnly sets the Checked state for the NamedBool(s) with the
// given name to true and all others to false.
func (nba *NamedBoolArray) SetOnly(name string) {
	name = strings.TrimPrefix(name, nba.prefix)
	nba.mu.Lock()
	for _, nb := range nba.data {
		nb.Checked = (nb.Name == name)
	}
	nba.mu.Unlock()
}

func (nba *NamedBoolArray) isCheckedLocked(name string) bool {
	name = strings.TrimPrefix(name, nba.prefix)
	for _, nb := range nba.data {
		if nb.Checked && nb.Name == name {
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
	sb.WriteString("&NamedBoolArray{")
	nba.mu.RLock()
	for i, nb := range nba.data {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(nb.String())
	}
	nba.mu.RUnlock()
	sb.WriteByte('}')
	return sb.String()
}

func (nba *NamedBoolArray) radioList(rq *Request, fn InputTextFn) (rl []Radio) {
	nba.mu.RLock()
	rl = make([]Radio, len(nba.data))
	for i, nb := range nba.data {
		rl[i] = Radio{
			nba:       nba,
			rq:        rq,
			fn:        fn,
			NamedBool: *nb,
		}
	}
	nba.mu.RUnlock()
	return
}

func (nba *NamedBoolArray) radioEventFn(rq *Request, jid, evt, val string, fn InputTextFn) (err error) {
	if evt == "input" && val != "" && strings.HasPrefix(jid, nba.prefix) {
		var v bool
		if v, err = strconv.ParseBool(val); err == nil {
			name := strings.TrimPrefix(jid, nba.prefix)
			nba.mu.Lock()
			for _, nb := range nba.data {
				nb.Checked = v && (nb.Name == name)
			}
			nba.mu.Unlock()
			rq.SetBoolValue(jid, v)
			if fn != nil {
				err = fn(rq, name)
			}
		}
	}
	return
}
