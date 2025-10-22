package jaws

import (
	"testing"
)

func TestRequest_Td(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	want := `<td id="Jid.1">inner</td>`
	rq.Td("inner")
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Td() = %q, want %q", got, want)
	}
}
