package what

import "strings"

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=What
type What uint8

const (
	None What = iota
	// Commands not associated with an Element
	Reload
	Redirect
	Alert
	Hide  // Set the "hidden" attribute on a set of elements
	Show  // Remove the "hidden" attribute from a set of elements
	Order // Re-order a set of elements
	// Element manipulation
	Inner   // Set the elements inner HTML
	Delete  // Delete the element
	Replace // Replace the element with new HTML
	Remove  // Remove child element
	Insert  // Insert child element
	Append  // Append child element
	SAttr
	RAttr
	SClass
	RClass
	Value
	// Element events
	Input
	Click
	// Meta events
	Trigger
	Hook
	Disregard
)

func (w What) IsCommand() bool {
	return w <= Order
}

func Parse(s string) What {
	if s != "" {
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
	}
	return None
}
