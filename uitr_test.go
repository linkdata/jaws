package jaws

import (
	"testing"
)

func TestRequest_Tr(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	want := `<tr id="Jid.1">inner</tr>`
	if got := string(rq.Tr("inner")); got != want {
		t.Errorf("Request.Tr() = %q, want %q", got, want)
	}
}
