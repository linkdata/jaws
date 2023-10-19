package jaws

import (
	"testing"
)

func TestRequest_Div(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<div id="Jid.1">inner</div>`
	if got := string(rq.Div("inner")); got != want {
		t.Errorf("Request.Div() = %q, want %q", got, want)
	}
}
