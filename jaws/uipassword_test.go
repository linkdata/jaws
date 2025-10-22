package jaws

import (
	"testing"
)

func TestRequest_Password(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	ts := newTestSetter("")
	want := `<input id="Jid.1" type="password">`
	rq.Password(ts)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Password() = %q, want %q", got, want)
	}
}
