package ui

import "testing"

func TestValidHTMLNumber(t *testing.T) {
	tests := []struct {
		text string
		ok   bool
	}{
		{"0", true},
		{"-0", true},
		{"00", true},
		{"1", true},
		{"-1", true},
		{".5", true},
		{"-.5", true},
		{"1.0", true},
		{"1e3", true},
		{"1E+3", true},
		{"1e-3", true},
		{"-1.25e+3", true},
		{"", false},
		{"-", false},
		{"+1", false},
		{" 1", false},
		{"1 ", false},
		{".", false},
		{"-.", false},
		{"1.", false},
		{"1e", false},
		{"1e+", false},
		{"1e-", false},
		{"1_0", false},
		{"0x1p0", false},
		{"NaN", false},
		{"Inf", false},
		{"Infinity", false},
		{"１", false},
		{"−1", false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if ok := validHTMLNumber(tt.text); ok != tt.ok {
				t.Fatalf("validHTMLNumber(%q) = %v, want %v", tt.text, ok, tt.ok)
			}
		})
	}
}

func FuzzValidHTMLNumber(f *testing.F) {
	for _, seed := range []string{"0", "-.5e+2", "1.", "1e999999", "NaN"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		_ = validHTMLNumber(text)
	})
}
