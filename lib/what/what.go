package what

// What identifies a JaWS wire protocol command or event.
//
//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=What
type What uint8

const (
	invalid What = iota

	// Commands not associated with an Element
	Update   // Used for update scheduling
	Reload   // Tells browser to reload the current URL
	Redirect // Tells browser to load another URL
	Alert    // Display (if using Bootstrap) an alert message
	Order    // Re-order a set of elements
	Call     // Call JavaScript function
	Set      // Set JavaScript variable (JSON path + tab char + JSON data)

	separator

	// Element manipulation
	Inner   // Set the elements inner HTML
	Delete  // Delete the element
	Replace // Replace the element with new HTML
	Remove  // Remove child element (Data identifies child; Jid identifies parent)
	Insert  // Insert child element
	Append  // Append child element
	SAttr   // Set element attribute
	RAttr   // Remove element attribute
	SClass  // Set element class
	RClass  // Remove element class
	Value   // Set element value
	// Element input events
	Input
	Click
	ContextMenu
	// Testing
	Hook // Calls event handler synchronously
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
		if w := What(i); w.IsValid() && s == _What_name[_What_index[i]:_What_index[i+1]] { // #nosec G115
			return w
		}
	}
	return invalid
}
