package key

import (
	"math"
	"testing"
)

// TestRoundTrip asserts the inverse relationship the package guarantees:
// Parse(Key(k).String()) recovers k with an empty tail across the full uint64
// domain (keys come from a CSPRNG, so large values near math.MaxUint64 are
// realistic), and that String and Append produce identical text.
func TestRoundTrip(t *testing.T) {
	for _, k := range []Key{0, 1, 32, 0x0123456789abcdef, math.MaxUint64} {
		s := k.String()
		if got := string(Append(nil, k)); got != s {
			t.Fatalf("Append(%d) = %q, String() = %q; want equal", uint64(k), got, s)
		}
		got, tail := Parse(s)
		if got != k || tail != "" {
			t.Fatalf("Parse(%q) = %d, %q; want %d, %q", s, uint64(got), tail, uint64(k), "")
		}
	}
}

// TestParseCaseInsensitive pins the documented asymmetry: Parse decodes base-32
// case-insensitively while String emits only lowercase, so an uppercase prefix
// parses but does not round-trip to its own text.
func TestParseCaseInsensitive(t *testing.T) {
	upper, tail := Parse("1A")
	if tail != "" {
		t.Fatalf("Parse(%q) tail = %q, want empty", "1A", tail)
	}
	lower, _ := Parse("1a")
	if upper != lower {
		t.Fatalf("Parse(%q) = %d, Parse(%q) = %d; want equal", "1A", uint64(upper), "1a", uint64(lower))
	}
	if got := upper.String(); got != "1a" {
		t.Fatalf("Parse(%q).String() = %q, want lowercase %q", "1A", got, "1a")
	}
}

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
		tail string
	}{
		{name: "blank", in: "", want: 0},
		{name: "one", in: "1", want: 1},
		{name: "invalid", in: "-1", want: 0},
		{name: "trailing-path", in: "2/noscript", want: 2, tail: "/noscript"},
		{name: "empty-trailing-path", in: "2/", want: 2, tail: "/"},
		{name: "base32", in: "10", want: 32},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, tail := Parse(tt.in)
			if got != tt.want || tail != tt.tail {
				t.Fatalf("Parse(%q) = %v, %q want %v, %q", tt.in, got, tail, tt.want, tt.tail)
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
