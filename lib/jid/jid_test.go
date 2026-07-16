package jid

import (
	"fmt"
	"math"
	"testing"
)

func TestParseJid(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want Jid
	}{
		{"zero", Prefix + "0", Invalid},
		{"one", Prefix + "1", 1},
		{"leading zero", Prefix + "01", Invalid},
		{"leading plus", Prefix + "+1", Invalid},
		{"negative zero", Prefix + "-0", Invalid},
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
				t.Errorf("ParseString(%q) = %v: IsValid() true but equals Invalid", tt.arg, got)
			}
			if !got.IsValid() && got != Invalid {
				t.Errorf("ParseString(%q) = %v: not valid but not equal to Invalid", tt.arg, got)
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

func TestJid_AppendVariants(t *testing.T) {
	tests := []struct {
		name       string
		jid        Jid
		wantInt    string
		wantAppend string
		wantQuote  string
	}{
		{"negative", -1, "", "", `""`},
		{"zero", 0, "", "", `""`},
		{"positive", 42, "42", Prefix + "42", `"` + Prefix + `42"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.jid.AppendInt(nil)); got != tt.wantInt {
				t.Errorf("AppendInt() = %q, want %q", got, tt.wantInt)
			}
			if got := string(tt.jid.AppendInt([]byte("x"))); got != "x"+tt.wantInt {
				t.Errorf("AppendInt() did not preserve dst prefix: %q", got)
			}
			if got := string(tt.jid.Append(nil)); got != tt.wantAppend {
				t.Errorf("Append() = %q, want %q", got, tt.wantAppend)
			}
			if got := string(tt.jid.AppendQuote(nil)); got != tt.wantQuote {
				t.Errorf("AppendQuote() = %q, want %q", got, tt.wantQuote)
			}
			// Every Append* variant must append to dst, never replace it.
			if got := string(tt.jid.Append([]byte("x"))); got != "x"+tt.wantAppend {
				t.Errorf("Append() did not preserve dst prefix: %q", got)
			}
			if got := string(tt.jid.AppendQuote([]byte("x"))); got != "x"+tt.wantQuote {
				t.Errorf("AppendQuote() did not preserve dst prefix: %q", got)
			}
		})
	}
}

// TestParseInt_NonCanonical pins how the numeric parser treats non-canonical
// integer text. ParseString independently accepts only canonical HTML IDs.
func TestParseInt_NonCanonical(t *testing.T) {
	tests := []struct {
		arg  string
		want Jid
	}{
		{"1", 1},
		{"+1", 1}, // leading plus accepted, normalizes to Jid(1)
		{"01", 1}, // leading zero accepted, normalizes to Jid(1)
		{"0", 0},  // the whole-request id
		{"-0", 0}, // negative zero is zero
		{"-1", Invalid},
		{"", Invalid},
		{" 1", Invalid}, // surrounding space rejected
		{"1 ", Invalid},
		{"1\n", Invalid}, // trailing newline rejected
		{"0x1", Invalid}, // base-10 only
		{"1_000", Invalid},
		{"99999999999999999999999", Invalid}, // overflow
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			if got := ParseInt(tt.arg); got != tt.want {
				t.Errorf("ParseInt(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestParseString_Canonical(t *testing.T) {
	tests := []struct {
		arg  string
		want Jid
	}{
		{"", 0},
		{Prefix + "1", 1},
		{Prefix + "10", 10},
		{Prefix + fmt.Sprint(int64(math.MaxInt64)), Jid(math.MaxInt64)},
		{Prefix + "0", Invalid},
		{Prefix + "01", Invalid},
		{Prefix + "+1", Invalid},
		{Prefix + "-0", Invalid},
		{Prefix + "-1", Invalid},
		{Prefix, Invalid},
		{"1", Invalid},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			if got := ParseString(tt.arg); got != tt.want {
				t.Fatalf("ParseString(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func FuzzParseJid(f *testing.F) {
	f.Add("")
	f.Add("0")
	f.Add("1")
	f.Add("+1")
	f.Add("01")
	f.Add("-0")
	f.Add("-1")
	f.Add(" 1")
	f.Add("1 ")
	f.Add("1\n")
	f.Add("0x1")
	f.Add("1_000")
	f.Add("99999999999999999999999")
	f.Add(Prefix)
	f.Add(Prefix + "0")
	f.Add(Prefix + "1")
	f.Add(Prefix + "-1")
	f.Add(fmt.Sprint(uint64(math.MaxInt64 + 1)))
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 256 {
			s = s[:256]
		}

		gotInt := ParseInt(s)
		assertParsedJid(t, "ParseInt", s, gotInt)

		gotPrefixed := ParseString(Prefix + s)
		assertParsedJid(t, "ParseString(Prefix+input)", Prefix+s, gotPrefixed)
		if gotPrefixed.IsValid() && (gotPrefixed < 1 || gotPrefixed.String() != Prefix+s) {
			t.Fatalf("ParseString(%q) returned non-canonical %v", Prefix+s, gotPrefixed)
		}

		gotString := ParseString(s)
		assertParsedJid(t, "ParseString", s, gotString)

		assertJidRoundTrip(t, "ParseInt", s, gotInt)
		assertJidRoundTrip(t, "ParseString", s, gotString)
	})
}

func assertParsedJid(t *testing.T, parser, input string, got Jid) {
	t.Helper()
	if got != Invalid && !got.IsValid() {
		t.Fatalf("%s(%q) returned non-canonical invalid Jid %v", parser, input, got)
	}
}

func assertJidRoundTrip(t *testing.T, parser, input string, got Jid) {
	t.Helper()
	if !got.IsValid() {
		return
	}
	if reparsed := ParseString(got.String()); reparsed != got {
		t.Fatalf("%s(%q) = %v, but ParseString(%q) = %v", parser, input, got, got.String(), reparsed)
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
			if got := string(tt.jid.AppendStartTagAttr(nil, tt.name)); got != tt.want {
				t.Errorf("Jid.AppendStartTagAttr() = %q, want %q", got, tt.want)
			}
			if got := string(tt.jid.AppendStartTagAttr([]byte("x"), tt.name)); got != "x"+tt.want {
				t.Errorf("Jid.AppendStartTagAttr() did not preserve dst prefix: %q", got)
			}
		})
	}
}
