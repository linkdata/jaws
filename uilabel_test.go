package jaws

import (
	"testing"
)

func TestRequest_Label(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<label id="Jid.1">inner</label>`
	rq.Label("inner")
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Label() = %q, want %q", got, want)
	}
}
