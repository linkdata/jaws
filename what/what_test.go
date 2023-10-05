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
		{"blank is None", "", Update},
		{"None", "None", Update},
		{"Inner", "Inner", Inner},
		{"inner", "inner", Inner},
		{"innerr", "innerr", invalid},
		{"last", lastWhat.String(), lastWhat},
		{"newline", "\n", invalid},
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
	if invalid.IsValid() {
		t.Fail()
	}
	if !Update.IsValid() {
		t.Fail()
	}
	if invalid.IsCommand() {
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
		{"None", Update, "None"},
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
