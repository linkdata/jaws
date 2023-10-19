package jaws

import (
	"testing"
)

func TestRequest_Button(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<button id="Jid.1" type="button">inner</button>`
	if got := string(rq.Button("inner")); got != want {
		t.Errorf("Request.Button() = %q, want %q", got, want)
	}
}
