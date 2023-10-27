package jaws

import (
	"testing"
)

func TestRequest_Tr(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<tr id="Jid.1">inner</tr>`
	rq.Tr("inner")
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Tr() = %q, want %q", got, want)
	}
}
