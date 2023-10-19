package jaws

import (
	"errors"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Checkbox(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter(true)
	want := `<input id="Jid.1" type="checkbox" checked>`
	if got := string(rq.Checkbox(ts)); got != want {
		t.Errorf("Request.Checkbox() = %q, want %q", got, want)
	}

	val := false
	rq.inCh <- wsMsg{Data: "false", Jid: 1, What: what.Input}
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	select {
	case <-tmr.C:
		t.Error("timeout")
	case <-ts.setCalled:
	}
	if ts.Get() != val {
		t.Error(ts.Get(), "!=", val)
	}
	select {
	case s := <-rq.outCh:
		t.Errorf("%q", s)
	default:
	}

	val = true
	ts.Set(val)
	rq.Dirty(ts)
	select {
	case <-tmr.C:
		t.Error("timeout waiting for Value")
	case s := <-rq.outCh:
		if s != "Value\tJid.1\t\"true\"\n" {
			t.Error("wrong Value")
		}
	}
	if ts.Get() != val {
		t.Error("not set")
	}
	if ts.SetCount() != 1 {
		t.Error("SetCount", ts.SetCount())
	}

	rq.inCh <- wsMsg{Data: "omg", Jid: 1, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout waiting for Alert")
	case s := <-rq.outCh:
		if s != "Alert\t\t\"danger\\nstrconv.ParseBool: parsing &#34;omg&#34;: invalid syntax\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}

	ts.err = errors.New("meh")
	rq.inCh <- wsMsg{Data: "true", Jid: 1, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout waiting for Alert")
	case s := <-rq.outCh:
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}
}
