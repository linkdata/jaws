package jaws

import (
	"testing"
)

func TestRequest_Td(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<td id="Jid.1">inner</td>`
	if got := string(rq.Td("inner")); got != want {
		t.Errorf("Request.Td() = %q, want %q", got, want)
	}
}
