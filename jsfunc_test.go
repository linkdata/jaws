package jaws

import (
	"strings"
	"testing"
)

func TestJsCall(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{}

	var sb strings.Builder
	elem := rq.NewElement(tss)
	th.Equal(elem.Jid(), Jid(1))
	err := elem.JawsRender(&sb, nil)
	th.NoErr(err)

	elem.Jaws.JsCall(tss, "somefn", "1.3")

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		got := msg.Format()
		th.Equal(got, "Call\tJid.1\tsomefn=1.3\n")
	}
}
