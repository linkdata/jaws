package jaws

import (
	"errors"
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Checkbox(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	ts := newTestSetter(true)
	want := `<input id="Jid.1" type="checkbox" checked>`
	rq.Checkbox(ts)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Checkbox() = %q, want %q", got, want)
	}

	val := false
	rq.InCh <- wsMsg{Data: "false", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case <-ts.setCalled:
	}
	if ts.Get() != val {
		t.Error(ts.Get(), "!=", val)
	}
	select {
	case s := <-rq.OutCh:
		t.Errorf("%q", s)
	default:
	}

	val = true
	ts.Set(val)
	rq.Dirty(ts)
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.OutCh:
		s := msg.Format()
		if s != "Value\tJid.1\t\"true\"\n" {
			t.Errorf("%q", s)
		}
	}
	if ts.Get() != val {
		t.Error("not set")
	}
	if ts.SetCount() != 1 {
		t.Error("SetCount", ts.SetCount())
	}

	rq.InCh <- wsMsg{Data: "omg", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.OutCh:
		s := msg.Format()
		if s != "Alert\t\t\"danger\\nstrconv.ParseBool: parsing &#34;omg&#34;: invalid syntax\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}

	ts.err = errors.New("meh")
	rq.InCh <- wsMsg{Data: "true", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.OutCh:
		s := msg.Format()
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}
}
