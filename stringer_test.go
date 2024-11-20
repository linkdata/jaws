package jaws

import (
	"reflect"
	"testing"
)

func TestStringer(t *testing.T) {
	var pnil any
	txt := any("text")
	num := any(int(123))
	tests := []struct {
		name string
		arg  *any
		want string
	}{
		{
			name: "nil",
			arg:  nil,
			want: "<nil>",
		},
		{
			name: "pointer to nil",
			arg:  &pnil,
			want: "<nil>",
		},
		{
			name: "text",
			arg:  &txt,
			want: "text",
		},
		{
			name: "num",
			arg:  &num,
			want: "123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Stringer(tt.arg).String(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stringer() = %v, want %v", got, tt.want)
			}
		})
	}
}
