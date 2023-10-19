package jaws

import (
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Textarea(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ss := newTestSetter("foo")
	want := `<textarea id="Jid.1">foo</textarea>`
	if got := string(rq.Textarea(ss)); got != want {
		t.Errorf("Request.Textarea() = %q, want %q", got, want)
	}
	rq.inCh <- wsMsg{Data: "bar", Jid: 1, What: what.Input}
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	select {
	case <-tmr.C:
		t.Fail()
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
	case <-tmr.C:
		t.Fail()
	case s := <-rq.outCh:
		if s != "Inner\tJid.1\t\"quux\"\n" {
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
