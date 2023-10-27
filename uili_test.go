package jaws

import (
	"testing"
)

func TestRequest_Li(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<li id="Jid.1">inner</li>`
	rq.Li("inner")
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Li() = %q, want %q", got, want)
	}
}
