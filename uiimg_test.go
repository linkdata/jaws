package jaws

import (
	"testing"
)

func TestRequest_Img(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<img id="Jid.1" src="inner">`
	if got := string(rq.Img("inner")); got != want {
		t.Errorf("Request.Img() = %q, want %q", got, want)
	}
	want = `<img id="Jid.2" src="inner">`
	if got := string(rq.Img("\"inner\"")); got != want {
		t.Errorf("Request.Img() = %q, want %q", got, want)
	}
}
