package what

import "strings"

//go:generate stringer -type=What
type What uint8

const (
	None What = iota
	Inner
	Remove
	Insert
	Append
	Replace
	Reload
	SAttr
	RAttr
	Value
	Trigger
	Hook
	Input
	Click
)

func Parse(s string) What {
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
	return None
}
