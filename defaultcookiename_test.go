package jaws

import (
	"path"
	"testing"
)

func Test_makeCookieName(t *testing.T) {
	tests := []struct {
		name       string
		exename    string
		wantCookie string
	}{
		{"empty string", "", "jaws"},
		{"Simple", "Simple", "Simple"},
		{"suffix.ed", "suffix.ed", "suffix"},
		{"path", path.Join("c:", "path", "file.ext"), "file"},
		{"invalid chars", " !??_", "jaws"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotCookie := makeCookieName(tt.exename); gotCookie != tt.wantCookie {
				t.Errorf("makeCookieName() = %v, want %v", gotCookie, tt.wantCookie)
			}
		})
	}
}
