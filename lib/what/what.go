package what

// What identifies a JaWS wire protocol command or event.
//
//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=What
type What uint8

const (
	invalid What = iota

	// Commands not associated with an Element

	// Update schedules dirty element processing.
	Update
	// Reload tells the browser to reload the current URL.
	Reload
	// Redirect tells the browser to load another URL.
	Redirect
	// Alert displays an alert message when the client UI supports it.
	Alert
	// Order reorders a set of elements.
	Order
	// Call calls a JavaScript function.
	Call
	// Set sets a JavaScript variable as path=json.
	Set

	separator

	// Element manipulation

	// Inner sets the element's inner HTML.
	Inner
	// Delete deletes the element.
	Delete
	// Replace replaces the element with new HTML.
	Replace
	// Remove removes a child element; Data identifies child and Jid identifies parent.
	Remove
	// Insert inserts a child element.
	Insert
	// Append appends a child element.
	Append
	// SAttr sets an element attribute.
	SAttr
	// RAttr removes an element attribute.
	RAttr
	// SClass sets an element class.
	SClass
	// RClass removes an element class.
	RClass
	// Value sets an element value.
	Value

	// Element input events

	// Input reports that an element value or input changed.
	Input
	// Click reports that an element was clicked.
	Click
	// ContextMenu reports that a context menu was requested on an element.
	ContextMenu

	// Synchronous event-handler invocation. A real protocol command (used by
	// broadcasts as well as tests), not a test-only artifact.

	// Hook calls the matching event handler synchronously.
	Hook
)

// IsCommand reports whether w is a non-element command.
func (w What) IsCommand() bool {
	return w < separator && w.IsValid()
}

// IsValid reports whether w is a known command or event.
func (w What) IsValid() bool {
	return w != invalid && w != separator
}

// Parse returns the [What] named by s.
//
// Matching is exact and case-sensitive: s must equal a command or event name as
// produced by [What.String], matching what the JaWS client sends on the wire. An
// empty string is treated as [Update]. Unknown strings, as well as the names of
// the internal boundary markers (which are not valid commands or events), return
// the invalid zero value.
func Parse(s string) What {
	if s == "" {
		return Update
	}
	for i := range len(_What_index) - 1 {
		if w := What(i); w.IsValid() && s == _What_name[_What_index[i]:_What_index[i+1]] { // #nosec G115 -- i ranges over [0, len(_What_index)-1), always within What's value range
			return w
		}
	}
	return invalid
}
