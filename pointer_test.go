package jaws

import (
	"strings"
	"testing"
)

func TestPointerKindString(t *testing.T) {
	tests := []struct {
		kind PointerKind
		want string
	}{
		{PointerDown, "down"},
		{PointerMove, "move"},
		{PointerUp, "up"},
		{PointerCancel, "cancel"},
		{PointerKind(99), "PointerKind(99)"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Fatalf("%v.String() = %q, want %q", uint8(tt.kind), got, tt.want)
		}
	}
}

func TestParsePointerData(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Pointer
		wantRem string
		wantOK  bool
	}{
		{
			name:   "down without modifiers",
			in:     "down 10 20 0 0 1 draw",
			want:   Pointer{Name: "draw", X: 10, Y: 20, Kind: PointerDown, Button: 0, Buttons: PointerButtonPrimary},
			wantOK: true,
		},
		{
			name:    "move with modifiers and route",
			in:      "move 10.5 20.25 5 -1 1 draw\tJid.1\tJid.2",
			want:    Pointer{Name: "draw", X: 10.5, Y: 20.25, Kind: PointerMove, Button: -1, Buttons: PointerButtonPrimary, Shift: true, Alt: true},
			wantRem: "Jid.1\tJid.2",
			wantOK:  true,
		},
		{
			name:   "up without name",
			in:     "up 10 20 2 0 0",
			want:   Pointer{X: 10, Y: 20, Kind: PointerUp, Button: 0, Control: true},
			wantOK: true,
		},
		{
			name:   "name with spaces",
			in:     "cancel 10 20 2 0 0 draw area",
			want:   Pointer{Name: "draw area", X: 10, Y: 20, Kind: PointerCancel, Button: 0, Control: true},
			wantOK: true,
		},
		{
			name:   "exponent coordinates",
			in:     "down 1e2 -2.5e-1 0 0 1 draw",
			want:   Pointer{Name: "draw", X: 100, Y: -0.25, Kind: PointerDown, Button: 0, Buttons: PointerButtonPrimary},
			wantOK: true,
		},
		{
			name:   "invalid kind",
			in:     "bad 10 20 0 0 1 draw",
			wantOK: false,
		},
		{
			name:   "invalid x",
			in:     "down bad 20 0 0 1 draw",
			wantOK: false,
		},
		{
			name:   "invalid y",
			in:     "down 10 bad 0 0 1 draw",
			wantOK: false,
		},
		{
			name:   "invalid keystate",
			in:     "down 10 20 bad 0 1 draw",
			wantOK: false,
		},
		{
			name:   "invalid button",
			in:     "down 10 20 0 bad 1 draw",
			wantOK: false,
		},
		{
			name:   "invalid buttons",
			in:     "down 10 20 0 0 bad draw",
			wantOK: false,
		},
		{
			name:   "non-finite x",
			in:     "down NaN 20 0 0 1 draw",
			wantOK: false,
		},
		{
			name:   "non-finite y",
			in:     "down 10 +Inf 0 0 1 draw",
			wantOK: false,
		},
		{
			name:   "too few fields",
			in:     "down 10 20 0 0",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rem, ok := parsePointerData(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Fatalf("pointer = %+v, want %+v", got, tt.want)
			}
			if rem != tt.wantRem {
				t.Fatalf("after = %q, want %q", rem, tt.wantRem)
			}
		})
	}
}

func TestPointerString(t *testing.T) {
	got := (Pointer{
		Name:    "draw",
		X:       1.25,
		Y:       2.5,
		Button:  -1,
		Buttons: PointerButtonPrimary,
		Kind:    PointerMove,
		Shift:   true,
		Control: true,
		Alt:     true,
	}).String()
	want := "move 1.25 2.5 7 -1 1 draw"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func Fuzz_parsePointerData(f *testing.F) {
	f.Add("down 1 2 0 0 1 name")
	f.Add("move 1 2 5 -1 1 name")
	f.Add("move 1 2 5 -1 1 name\tJid.1\tJid.2")
	f.Add("up 1.5 2.25 0 0 0 name")
	f.Add("cancel 1e2 -2.5e-1 0 0 0")
	f.Add("bad 2 0 0 1 name")
	f.Fuzz(func(t *testing.T, in string) {
		ptr, after, ok := parsePointerData(in)
		if !ok {
			return
		}

		encoded := ptr.String()
		ptr2, after2, ok := parsePointerData(encoded)
		if !ok {
			t.Fatalf("parsePointerData(Pointer.String()) failed: pointer=%+v encoded=%q", ptr, encoded)
		}
		if ptr2 != ptr || after2 != "" {
			t.Fatalf("parsePointerData(Pointer.String()) mismatch: pointer=%+v got=%+v after=%q", ptr, ptr2, after2)
		}

		roundtripInput := encoded
		if after != "" {
			roundtripInput += "\t" + after
		}
		ptr3, after3, ok := parsePointerData(roundtripInput)
		if !ok {
			t.Fatalf("parsePointerData(roundtrip input) failed: input=%q", roundtripInput)
		}
		if ptr3 != ptr || after3 != after {
			t.Fatalf("roundtrip mismatch: pointer=%+v/%+v after=%q/%q", ptr, ptr3, after, after3)
		}
	})
}

func Fuzz_pointerStringRoundTrip(f *testing.F) {
	f.Add("name", int32(1), int32(2), int32(-1), int32(1), uint8(PointerMove), true, false, true)
	f.Add("draw", int32(-1), int32(999), int32(0), int32(0), uint8(PointerUp), false, false, false)
	f.Fuzz(func(t *testing.T, name string, x, y, button, buttons int32, kindValue uint8, shift, control, alt bool) {
		name = strings.ReplaceAll(name, "\t", " ")
		kind := PointerKind(kindValue%4 + 1)
		ptr := Pointer{
			Name:    name,
			X:       float64(x) / 10,
			Y:       float64(y) / 10,
			Button:  int(button),
			Buttons: int(buttons),
			Kind:    kind,
			Shift:   shift,
			Control: control,
			Alt:     alt,
		}
		got, after, ok := parsePointerData(ptr.String())
		if !ok {
			t.Fatalf("parsePointerData(String()) failed: pointer=%+v", ptr)
		}
		if after != "" {
			t.Fatalf("expected no trailing data, got %q", after)
		}
		if got != ptr {
			t.Fatalf("roundtrip mismatch: want=%+v got=%+v", ptr, got)
		}
	})
}
