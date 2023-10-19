package jaws

import (
	"testing"
)

func TestRequest_Label(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<label id="Jid.1">inner</label>`
	if got := string(rq.Label("inner")); got != want {
		t.Errorf("Request.Label() = %q, want %q", got, want)
	}
}
