package jaws

import (
	"testing"
)

func TestRequest_Radio(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter(true)
	want := `<input id="Jid.1" type="radio" checked>`
	rq.Radio(ts)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Radio() = %q, want %q", got, want)
	}
}
