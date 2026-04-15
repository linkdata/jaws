package jaws

import "testing"

func TestParseClickData(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Click
		wantRem string
		wantOK  bool
	}{
		{
			name:   "legacy without modifiers",
			in:     "save\t10\t20",
			want:   Click{Name: "save", X: 10, Y: 20},
			wantOK: true,
		},
		{
			name:    "with modifiers and route",
			in:      "save\t10\t20\ttrue\tfalse\ttrue\tJid.1\tJid.2",
			want:    Click{Name: "save", X: 10, Y: 20, Shift: true, Alt: true},
			wantRem: "Jid.1\tJid.2",
			wantOK:  true,
		},
		{
			name:    "route without modifiers",
			in:      "save\t10\t20\tJid.1",
			want:    Click{Name: "save", X: 10, Y: 20},
			wantRem: "Jid.1",
			wantOK:  true,
		},
		{
			name:    "one modifier and route",
			in:      "save\t10\t20\ttrue\tJid.1",
			want:    Click{Name: "save", X: 10, Y: 20, Shift: true},
			wantRem: "Jid.1",
			wantOK:  true,
		},
		{
			name:   "invalid x",
			in:     "save\tbad\t20",
			wantOK: false,
		},
		{
			name:   "invalid y",
			in:     "save\t10\tbad",
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
	got := (Click{Name: "x", X: 1, Y: 2, Shift: true, Control: false, Alt: true}).String()
	want := "x\t1\t2\ttrue\tfalse\ttrue"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
