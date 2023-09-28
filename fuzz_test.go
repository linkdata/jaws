//go:build go1.18
// +build go1.18

package jaws

import (
	"bytes"
	"strings"
	"testing"
)

func Fuzz_wsParse(f *testing.F) {
	f.Add([]byte("\n\n"))
	f.Add([]byte("Inner\nJid.0\ndata\nline"))
	f.Fuzz(func(t *testing.T, a []byte) {
		if msg, ok := wsParse(a); ok {
			b := msg.Append(nil)
			if !bytes.Equal(a, b) {
				t.Errorf("%q != %q", string(a), string(b))
			}
		} else {
			if len(a) > 0 && a[0] != '\n' && strings.Count(string(a), "\n") > 1 {
				t.Errorf("%q => nil", a)
			}
		}
	})
}
