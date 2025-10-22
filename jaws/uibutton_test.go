package jaws

import (
	"testing"
)

func TestRequest_Button(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	want := `<button id="Jid.1" type="button">inner</button>`
	rq.Button("inner")
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Button() = %q, want %q", got, want)
	}
}
