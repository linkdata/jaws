package what

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want What
	}{
		{"blank is None", "", None},
		{"Inner", "Inner", Inner},
		{"inner", "inner", Inner},
		{"inners", "inners", None},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.arg); got != tt.want {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name string
		arg  What
		want string
	}{
		{"None", None, "None"},
		{"Inner", Inner, "Inner"},
		{"unknown", What(len(_What_index)), fmt.Sprintf("What(%d)", len(_What_index))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.arg.String(); got != tt.want {
				t.Errorf("%v.String() = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}
