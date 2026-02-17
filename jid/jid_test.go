package jid

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestParseJid(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want Jid
	}{
		{"zero", Prefix + "0", 0},
		{"one", Prefix + "1", 1},
		{"negative", Prefix + "-1", Invalid},
		{"empty string", "", 0},
		{"random text", "hello, world!", Invalid},
		{"missing number", Prefix, Invalid},
		{"overflow", Prefix + fmt.Sprint(uint64(math.MaxInt64+1)), Invalid},
		{"spaces", Prefix + " 1", Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseString(tt.arg)
			if got != tt.want {
				t.Errorf("ParseJid() = %v, want %v", got, tt.want)
			}
			if got.IsValid() && got == Invalid {
				t.Fail()
			}
			if !got.IsValid() && got != Invalid {
				t.Fail()
			}
		})
	}
}

func TestJid_String(t *testing.T) {
	tests := []struct {
		name string
		jid  Jid
		want string
	}{
		{"negative", -1, ""},
		{"zero", 0, ""},
		{"one", 1, Prefix + "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.jid.String(); got != tt.want {
				t.Errorf("Jid.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJid_AppendStartTagAttr(t *testing.T) {
	tests := []struct {
		name string
		jid  Jid
		want string
	}{
		{"zero", 0, "<zero"},
		{"one", 1, `<one id="` + Prefix + `1"`},
		{"negative", -1, "<negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.jid.AppendStartTagAttr(nil, tt.name)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Jid.AppendStartTagAttr() = %q, want %q", got, tt.want)
			}
		})
	}
}
