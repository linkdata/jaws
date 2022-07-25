//go:build go1.18
// +build go1.18

package jaws

import (
	"strings"
	"testing"
)

func Fuzz_wsParse(f *testing.F) {
	f.Add([]byte(" \n\n"))
	f.Add([]byte("elem\nwhat\ndata\nline"))
	f.Fuzz(func(t *testing.T, a []byte) {
		if msg := wsParse(a); msg != nil {
			txt := msg.Format()
			if string(a) != txt {
				t.Errorf("%q != %q", string(a), txt)
			}
		} else {
			if len(a) > 0 && a[0] != '\n' && strings.Count(string(a), "\n") > 1 {
				t.Errorf("%q => nil", a)
			}
		}
	})
}
