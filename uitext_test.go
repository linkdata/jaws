package jaws

import (
	"errors"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Text(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ss := newTestStringSetter("foo")
	want := `<input id="Jid.1" type="text" value="foo">`
	if got := string(rq.Text(ss)); got != want {
		t.Errorf("Request.Text() = %q, want %q", got, want)
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
		t.Error("timeout waiting for Value")
	case s := <-rq.outCh:
		if s != "Value\tJid.1\t\"quux\"\n" {
			t.Error("wrong Value")
		}
	}
	if ss.Get() != "quux" {
		t.Error("not quux")
	}
	if ss.SetCount() != 1 {
		t.Error("SetCount", ss.SetCount())
	}
	ss.err = errors.New("meh")
	rq.inCh <- wsMsg{Data: "omg", Jid: 1, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout waiting for Alert")
	case s := <-rq.outCh:
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}
}
