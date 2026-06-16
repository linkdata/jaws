package key

import "testing"

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

func TestParse(t *testing.T) {
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
			if got := Parse(tt.in); got != tt.want {
				t.Fatalf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestAppend(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []byte
		key  Key
		want string
	}{
		{name: "zero", in: []byte("jaws/"), key: 0, want: "jaws/"},
		{name: "one", in: []byte("jaws/"), key: 1, want: "jaws/1"},
		{name: "base32", in: []byte("jaws/"), key: 32, want: "jaws/10"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Append(tt.in, tt.key)); got != tt.want {
				t.Fatalf("Append() = %q, want %q", got, tt.want)
			}
		})
	}
}
