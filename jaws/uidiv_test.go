package jaws

import (
	"testing"
)

func TestRequest_Div(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	want := `<div id="Jid.1">inner</div>`
	rq.Div("inner")
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Div() = %q, want %q", got, want)
	}
}
