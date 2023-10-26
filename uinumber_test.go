package jaws

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Number(t *testing.T) {
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter(float64(1.2))
	want := fmt.Sprintf(`<input id="Jid.1" type="number" value="%v">`, ts.Get())
	rq.Number(ts)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Number() = %q, want %q", got, want)
	}

	val := float64(2.3)
	rq.inCh <- wsMsg{Data: fmt.Sprint(val), Jid: 1, What: what.Input}
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

	val = 3.4
	ts.Set(val)
	rq.Dirty(ts)
	select {
	case <-tmr.C:
		t.Error("timeout waiting for Value")
	case s := <-rq.outCh:
		if s != fmt.Sprintf("Value\tJid.1\t\"%v\"\n", val) {
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
		if s != "Alert\t\t\"danger\\nstrconv.ParseFloat: parsing &#34;omg&#34;: invalid syntax\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}

	ts.err = errors.New("meh")
	rq.inCh <- wsMsg{Data: fmt.Sprint(val), Jid: 1, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout waiting for Alert")
	case s := <-rq.outCh:
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}
}
