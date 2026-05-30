package what

import (
	"fmt"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	lastWhat := What(len(_What_index) - 2)
	tests := []struct {
		name string
		arg  string
		want What
	}{
		{"blank is Update", "", Update},
		{"Update", "Update", Update},
		{"Inner", "Inner", Inner},
		{"ContextMenu", "ContextMenu", ContextMenu},
		{"lowercase is not matched", "inner", invalid},
		{"innerr", "innerr", invalid},
		{"last", lastWhat.String(), lastWhat},
		{"newline", "\n", invalid},
		{"invalid marker name", "invalid", invalid},
		{"separator marker name", "separator", invalid},
		{"separator case-insensitive", "SEPARATOR", invalid},
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
		{"invalid", invalid, "invalid"},
		{"Inner", Inner, "Inner"},
		{"unknown", ^What(0), fmt.Sprintf("What(%d)", ^What(0))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.arg.String(); got != tt.want {
				t.Errorf("%v.String() = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	f.Add("")
	for i := range len(_What_index) - 1 {
		name := What(i).String()
		f.Add(name)
		f.Add(strings.ToLower(name))
		f.Add(strings.ToUpper(name))
	}
	f.Add("innerr")
	f.Add("\n")
	f.Add(" Update")
	f.Add("Update ")
	f.Add("ContextMenu\n")

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 128 {
			s = s[:128]
		}

		got := Parse(s)
		if s == "" {
			if got != Update {
				t.Fatalf("Parse(%q) = %v, want %v", s, got, Update)
			}
			return
		}
		if got != invalid && !got.IsValid() {
			t.Fatalf("Parse(%q) returned invalid non-zero command %v", s, got)
		}
		if !got.IsValid() {
			return
		}
		if got.String() != s {
			t.Fatalf("Parse(%q) = %v, but valid commands must match %q exactly", s, got, got.String())
		}
		if reparsed := Parse(got.String()); reparsed != got {
			t.Fatalf("Parse(%q) = %v, but Parse(%q) = %v", s, got, got.String(), reparsed)
		}
	})
}
