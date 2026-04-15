package jaws

import (
	"strings"
	"testing"
)

func TestParseClickData(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Click
		wantRem string
		wantOK  bool
	}{
		{
			name:   "without modifiers",
			in:     "10 20 0 save",
			want:   Click{Name: "save", X: 10, Y: 20},
			wantOK: true,
		},
		{
			name:    "with modifiers and route",
			in:      "10 20 5 save\tJid.1\tJid.2",
			want:    Click{Name: "save", X: 10, Y: 20, Shift: true, Alt: true},
			wantRem: "Jid.1\tJid.2",
			wantOK:  true,
		},
		{
			name:    "route without modifiers",
			in:      "10 20 0 save\tJid.1",
			want:    Click{Name: "save", X: 10, Y: 20},
			wantRem: "Jid.1",
			wantOK:  true,
		},
		{
			name:    "one modifier and route",
			in:      "10 20 2 save\tJid.1",
			want:    Click{Name: "save", X: 10, Y: 20, Control: true},
			wantRem: "Jid.1",
			wantOK:  true,
		},
		{
			name:   "name with spaces",
			in:     "10 20 2 save button",
			want:   Click{Name: "save button", X: 10, Y: 20, Control: true},
			wantOK: true,
		},
		{
			name:   "invalid x",
			in:     "bad 20 0 save",
			wantOK: false,
		},
		{
			name:   "invalid y",
			in:     "10 bad 0 save",
			wantOK: false,
		},
		{
			name:   "invalid keystate",
			in:     "10 20 bad save",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rem, ok := parseClickData(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Fatalf("click = %+v, want %+v", got, tt.want)
			}
			if rem != tt.wantRem {
				t.Fatalf("after = %q, want %q", rem, tt.wantRem)
			}
		})
	}
}

func TestClickString(t *testing.T) {
	got := (Click{Name: "x", X: 1, Y: 2, Shift: true, Control: true, Alt: true}).String()
	want := "1 2 7 x"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func Fuzz_parseClickData(f *testing.F) {
	f.Add("1 2 0 name")
	f.Add("1 2 5 name")
	f.Add("1 2 5 name\tJid.1\tJid.2")
	f.Add("1 2 0 name\tJid.1")
	f.Add("bad 2 0 name")
	f.Fuzz(func(t *testing.T, in string) {
		clk, after, ok := parseClickData(in)
		if !ok {
			return
		}

		encoded := clk.String()
		clk2, after2, ok := parseClickData(encoded)
		if !ok {
			t.Fatalf("parseClickData(Click.String()) failed: click=%+v encoded=%q", clk, encoded)
		}
		if clk2 != clk || after2 != "" {
			t.Fatalf("parseClickData(Click.String()) mismatch: click=%+v got=%+v after=%q", clk, clk2, after2)
		}

		roundtripInput := encoded
		if after != "" {
			roundtripInput += "\t" + after
		}
		clk3, after3, ok := parseClickData(roundtripInput)
		if !ok {
			t.Fatalf("parseClickData(roundtrip input) failed: input=%q", roundtripInput)
		}
		if clk3 != clk || after3 != after {
			t.Fatalf("roundtrip mismatch: click=%+v/%+v after=%q/%q", clk, clk3, after, after3)
		}
	})
}

func Fuzz_clickStringRoundTrip(f *testing.F) {
	f.Add("name", int32(1), int32(2), true, false, true)
	f.Add("button", int32(-1), int32(999), false, false, false)
	f.Fuzz(func(t *testing.T, name string, x int32, y int32, shift, control, alt bool) {
		name = strings.ReplaceAll(name, "\t", " ")
		clk := Click{
			Name:    name,
			X:       int(x),
			Y:       int(y),
			Shift:   shift,
			Control: control,
			Alt:     alt,
		}
		got, after, ok := parseClickData(clk.String())
		if !ok {
			t.Fatalf("parseClickData(String()) failed: click=%+v", clk)
		}
		if after != "" {
			t.Fatalf("expected no trailing data, got %q", after)
		}
		if got != clk {
			t.Fatalf("roundtrip mismatch: want=%+v got=%+v", clk, got)
		}
	})
}
