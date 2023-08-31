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
		{"empty string", "", 0},
		{"random text", "hello, world!", 0},
		{"missing number", JidPrefix, 0},
		{"negative number", JidPrefix + "-1", 0},
		{"overflow", JidPrefix + "42949672950", 0},
		{"spaces", JidPrefix + " 1", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseJid(tt.arg); got != tt.want {
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

func TestJid_AppendAttr(t *testing.T) {
	tests := []struct {
		name string
		jid  Jid
		want string
	}{
		{"zero", 0, ""},
		{"one", 1, ` id="` + JidPrefix + `1"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.jid.AppendAttr(nil)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Jid.AppendAttr() = %q, want %q", got, tt.want)
			}
		})
	}
}
