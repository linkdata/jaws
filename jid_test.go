package jaws

import (
	"reflect"
	"testing"
)

func TestParseJid(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want Jid
	}{
		{"zero", JidPrefix + "0", 0},
		{"one", JidPrefix + "1", 1},
		{"negative", JidPrefix + "-1", JidInvalid},
		{"empty string", "", 0},
		{"random text", "hello, world!", JidInvalid},
		{"missing number", JidPrefix, JidInvalid},
		{"overflow", JidPrefix + "42949672950", JidInvalid},
		{"spaces", JidPrefix + " 1", JidInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JidParseString(tt.arg); got != tt.want {
				t.Errorf("ParseJid() = %v, want %v", got, tt.want)
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
		{"one", 1, JidPrefix + "1"},
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
		{"one", 1, `<one id="` + JidPrefix + `1"`},
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
