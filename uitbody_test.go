package jaws

import (
	"testing"
)

func TestRequest_Tbody(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<tbody id="Jid.1"></tbody>`
	rq.Tbody(&testContainer{})
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Span() = %q, want %q", got, want)
	}
}
