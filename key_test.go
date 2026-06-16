package jaws

import (
	"errors"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
)

func TestKeyString(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  Key
		want string
	}{
		{name: "zero", key: 0, want: ""},
		{name: "one", key: 1, want: "1"},
		{name: "base32", key: 32, want: "10"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.key.String(); got != tt.want {
				t.Fatalf("Key.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseKey(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want Key
	}{
		{name: "blank", in: "", want: 0},
		{name: "one", in: "1", want: 1},
		{name: "invalid", in: "-1", want: 0},
		{name: "trailing-path", in: "2/noscript", want: 2},
		{name: "base32", in: "10", want: 32},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseKey(tt.in); got != tt.want {
				t.Fatalf("ParseKey(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestKeyIsNotUsableAsTag(t *testing.T) {
	if _, err := tag.TagExpand(nil, Key(1)); !errors.Is(err, tag.ErrIllegalTagType) {
		t.Fatalf("TagExpand(Key(1)) error = %v, want %v", err, tag.ErrIllegalTagType)
	}
}
