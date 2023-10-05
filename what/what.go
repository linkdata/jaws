package what

import "strings"

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=What
type What uint8

const (
	invalid What = iota
	// Commands not associated with an Element
	Reload   // Tells browser to reload the current URL
	Redirect // Tells browser to load another URL
	Alert    // Display (if using Bootstrap) an alert message
	Order    // Re-order a set of elements
	// Element manipulation
	Inner   // Set the elements inner HTML
	Delete  // Delete the element
	Replace // Replace the element with new HTML
	Remove  // Remove child element
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
	// Meta
	Update // Used for update scheduling
	Hook   // Calls event handler synchronously, used for tests
)

func (w What) IsCommand() bool {
	return w <= Order && w.IsValid()
}

func (w What) IsValid() bool {
	return w != invalid
}

func Parse(s string) What {
	if s == "" {
		return Update
	}
	for i := 0; i < len(_What_index)-1; i++ {
		if s == _What_name[_What_index[i]:_What_index[i+1]] {
			return What(i)
		}
	}
	for i := 0; i < len(_What_index)-1; i++ {
		if strings.EqualFold(s, _What_name[_What_index[i]:_What_index[i+1]]) {
			return What(i)
		}
	}
	return invalid
}
