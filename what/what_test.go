package what

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	lastWhat := What(len(_What_index) - 2)
	tests := []struct {
		name string
		arg  string
		want What
	}{
		{"blank is None", "", None},
		{"None", "None", None},
		{"Inner", "Inner", Inner},
		{"inner", "inner", Inner},
		{"innerr", "innerr", _Invalid},
		{"last", lastWhat.String(), lastWhat},
		{"newline", "\n", _Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.arg); got != tt.want {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCommandAndValid(t *testing.T) {
	if _Invalid.IsValid() {
		t.Fail()
	}
	if !None.IsValid() {
		t.Fail()
	}
	if _Invalid.IsCommand() {
		t.Fail()
	}
	if What(255).IsCommand() {
		t.Fail()
	}
	if !Alert.IsCommand() {
		t.Fail()
	}
	if !Reload.IsCommand() {
		t.Fail()
	}
	if !Redirect.IsCommand() {
		t.Fail()
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
		{"unknown", What(len(_What_index) + 44), fmt.Sprintf("What(%d)", len(_What_index)+44)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.arg.String(); got != tt.want {
				t.Errorf("%v.String() = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}
