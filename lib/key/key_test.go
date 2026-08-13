package key

import (
	"bytes"
	"math"
	"strconv"
	"strings"
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
	withTail, tail := Parse("1A/x")
	if withTail != lower || tail != "/x" {
		t.Fatalf("Parse(%q) = %d, %q; want %d, %q", "1A/x", uint64(withTail), tail, uint64(lower), "/x")
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
		// "0" is a valid base-32 literal but decodes to Key(0), the invalid key, so
		// it conflates with empty/invalid input and re-encodes to "" rather than "0".
		{name: "zero-literal", in: "0", want: 0},
		{name: "one", in: "1", want: 1},
		{name: "invalid", in: "-1", want: 0},
		{name: "trailing-path", in: "2/noscript", want: 2, tail: "/noscript"},
		{name: "empty-trailing-path", in: "2/", want: 2, tail: "/"},
		{name: "base32", in: "10", want: 32},
		{name: "empty-prefix", in: "/noscript", want: 0, tail: "/noscript"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, tail := Parse(tt.in)
			if got != tt.want || tail != tt.tail {
				t.Fatalf("Parse(%q) = %v, %q want %v, %q", tt.in, got, tail, tt.want, tt.tail)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	maxKey := strconv.FormatUint(math.MaxUint64, 32)
	for _, seed := range []string{
		"",
		"0",
		"1",
		"01",
		"10",
		"1A",
		"-1",
		"1Z",
		"/tail",
		"2/",
		"2/tail/more",
		"1A/tail",
		"1/\xff\x00",
		"\xff/tail",
		"1∕tail",
		maxKey,
		maxKey + "/tail",
		maxKey + "0",
		maxKey + "0/tail",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, s string) {
		prefix := s
		wantTail := ""
		if slashIdx := strings.IndexByte(s, '/'); slashIdx >= 0 {
			prefix = s[:slashIdx]
			wantTail = s[slashIdx:]
		}

		got, tail := Parse(s)
		if tail != wantTail {
			t.Fatalf("Parse(%q) tail = %q, want %q", s, tail, wantTail)
		}

		parsed, err := strconv.ParseUint(prefix, 32, 64)
		want := Key(0)
		if err == nil {
			want = Key(parsed)
		}
		if got != want {
			t.Fatalf("Parse(%q) key = %d, want %d (prefix %q, ParseUint error %v)", s, uint64(got), uint64(want), prefix, err)
		}
		if got == 0 {
			return
		}

		canonical := got.String()
		if wantCanonical := strconv.FormatUint(uint64(got), 32); canonical != wantCanonical {
			t.Fatalf("Key(%d).String() = %q, want canonical base-32 %q", uint64(got), canonical, wantCanonical)
		}
		if canonical != strings.ToLower(canonical) {
			t.Fatalf("Key(%d).String() = %q, want lowercase base-32", uint64(got), canonical)
		}
		if appended := string(Append(nil, got)); appended != canonical {
			t.Fatalf("Append(nil, %d) = %q, String() = %q; want equal", uint64(got), appended, canonical)
		}
		if reparsed, reparsedTail := Parse(canonical); reparsed != got || reparsedTail != "" {
			t.Fatalf("Parse(%q) = %d, %q; want %d, empty tail", canonical, uint64(reparsed), reparsedTail, uint64(got))
		}
	})
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

func FuzzKeyEncodingRoundTrip(f *testing.F) {
	f.Add(uint64(0), []byte(nil))
	f.Add(uint64(1), []byte{})
	f.Add(uint64(32), []byte("jaws/"))
	f.Add(uint64(math.MaxUint64), []byte{0, '/', 0xff})

	f.Fuzz(func(t *testing.T, raw uint64, prefix []byte) {
		key := Key(raw)
		canonical := key.String()
		if appended := string(Append(nil, key)); appended != canonical {
			t.Fatalf("Append(nil, %d) = %q, String() = %q; want equal", raw, appended, canonical)
		}

		wantCanonical := ""
		if key != 0 {
			wantCanonical = strconv.FormatUint(raw, 32)
		}
		if canonical != wantCanonical {
			t.Fatalf("Key(%d).String() = %q, want %q", raw, canonical, wantCanonical)
		}

		want := make([]byte, 0, len(prefix)+len(canonical))
		want = append(want, prefix...)
		want = append(want, canonical...)
		dst := make([]byte, len(prefix), len(prefix)+len(canonical))
		copy(dst, prefix)
		got := Append(dst, key)
		if len(got) != len(want) {
			t.Fatalf("len(Append(%q, %d)) = %d, want %d", prefix, raw, len(got), len(want))
		}
		if !bytes.Equal(got[:len(prefix)], prefix) {
			t.Fatalf("Append(%q, %d) prefix = %q, want preserved prefix %q", prefix, raw, got[:len(prefix)], prefix)
		}
		if suffix := string(got[len(prefix):]); suffix != canonical {
			t.Fatalf("Append(%q, %d) suffix = %q, want %q", prefix, raw, suffix, canonical)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("Append(%q, %d) = %q, want %q", prefix, raw, got, want)
		}

		if reparsed, tail := Parse(canonical); reparsed != key || tail != "" {
			t.Fatalf("Parse(%q) = %d, %q; want %d, empty tail", canonical, uint64(reparsed), tail, raw)
		}
	})
}
