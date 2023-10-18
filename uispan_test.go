package jaws

import (
	"testing"
)

func TestRequest_Span(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<span id="Jid.1">inner</span>`
	if got := string(rq.Span("inner")); got != want {
		t.Errorf("Request.Span() = %q, want %q", got, want)
	}
}
