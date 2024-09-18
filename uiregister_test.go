package jaws

import (
	"testing"
)

func TestRequestWriter_Register(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	item := &testUi{}
	jid := rq.Register(item)
	th.Equal(jid, Jid(1))
	e := rq.getElementByJid(jid)
	th.NoErr(e.JawsRender(nil, nil))
}
