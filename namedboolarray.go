package jaws

import "fmt"

// NamedBool stores the data required to support HTML 'select' elements
// and sets of HTML radio buttons.
type NamedBool struct {
	Value   string
	Text    string
	Checked bool
}

func (nb *NamedBool) String() string {
	return fmt.Sprintf("&{%q,%q,%v}", nb.Value, nb.Text, nb.Checked)
}

// NamedBoolArray is an array of pointers to NamedBool.
type NamedBoolArray []*NamedBool

// NewNamedBoolArray allocates an array of *NamedBool and returns a pointer to it.
func NewNamedBoolArray() *NamedBoolArray {
	array := make(NamedBoolArray, 0)
	return &array
}

// Add creates a new NamedBool and appends it's pointer to the NamedBoolArray.
func (nb *NamedBoolArray) Add(val, text string) {
	*nb = append(*nb, &NamedBool{Value: val, Text: text})
}

// Set sets all NamedBool's Checked to state in the NamedBoolArray that have the given Value.
func (nb *NamedBoolArray) Set(val string, state bool) {
	for _, so := range *nb {
		if so.Value == val {
			so.Checked = state
		}
	}
}

// Check sets all NamedBool's Checked to true in the NamedBoolArray that have the given Value.
func (nb *NamedBoolArray) Check(val string) {
	nb.Set(val, true)
}

func (nb *NamedBoolArray) String() string {
	b := make([]byte, 0, 17+len(*nb)*16)
	b = append(b, "&NamedBoolArray{"...)
	for i, v := range *nb {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(fmt.Sprint(v))...)
	}
	b = append(b, '}')
	return string(b)
}
