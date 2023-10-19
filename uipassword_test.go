package jaws

import (
	"testing"
)

func TestRequest_Password(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	ts := newTestSetter("")
	want := `<input id="Jid.1" type="password">`
	if got := string(rq.Password(ts)); got != want {
		t.Errorf("Request.Password() = %q, want %q", got, want)
	}
}
