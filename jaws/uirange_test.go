package jaws

import (
	"errors"
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Range(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter(float64(1))
	want := `<input id="Jid.1" type="range" value="1">`
	rq.Range(ts)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Range() = %q, want %q", got, want)
	}
	rq.inCh <- wsMsg{Data: "2.1", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case <-ts.setCalled:
	}
	if ts.Get() != 2.1 {
		t.Error(ts.Get())
	}
	select {
	case s := <-rq.outCh:
		t.Errorf("%q", s)
	default:
	}
	ts.Set(2.3)
	rq.Dirty(ts)
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Value\tJid.1\t\"2.3\"\n" {
			t.Error(s)
		}
	}
	if ts.Get() != 2.3 {
		t.Error(ts.Get())
	}
	if ts.SetCount() != 1 {
		t.Error("SetCount", ts.SetCount())
	}

	ts.err = errors.New("meh")
	rq.inCh <- wsMsg{Data: "3.4", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}

	if ts.Get() != 2.3 {
		t.Error(ts.Get())
	}
}
