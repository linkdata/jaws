package jaws

import (
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Textarea(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ss := newTestSetter("foo")
	want := `<textarea id="Jid.1">foo</textarea>`
	rq.Textarea(ss)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Textarea() = %q, want %q", got, want)
	}
	rq.inCh <- wsMsg{Data: "bar", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case <-ss.setCalled:
	}
	if ss.Get() != "bar" {
		t.Fail()
	}
	select {
	case s := <-rq.outCh:
		t.Errorf("%q", s)
	default:
	}
	ss.Set("quux")
	rq.Dirty(ss)
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		if s != "Value\tJid.1\t\"quux\"\n" {
			t.Fail()
		}
	}
	if ss.Get() != "quux" {
		t.Fail()
	}
	if ss.SetCount() != 1 {
		t.Fail()
	}
}
