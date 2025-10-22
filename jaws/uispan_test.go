package jaws

import (
	"testing"
)

func TestRequest_Span(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	want := `<span id="Jid.1">inner</span>`
	rq.Span("inner")
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Span() = %q, want %q", got, want)
	}
}
