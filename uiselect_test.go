package jaws

import (
	"errors"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type testNamedBoolArray struct {
	mu        deadlock.Mutex
	setCount  int
	setCalled chan struct{}
	err       error
	*NamedBoolArray
}

func (ts *testNamedBoolArray) JawsSetString(e *Element, val string) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if err = ts.err; err == nil {
		err = ts.NamedBoolArray.JawsSetString(e, val)
		ts.setCount++
		if ts.setCount == 1 {
			close(ts.setCalled)
		}
	}
	return
}

func TestRequest_Select(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	a := &testNamedBoolArray{
		setCalled:      make(chan struct{}),
		NamedBoolArray: NewNamedBoolArray(),
	}
	a.Add("1", "one")
	a.Add("2", "two")
	a.Set("1", true)

	wantHtml := "<select id=\"Jid.1\" disabled><option id=\"Jid.2\" value=\"1\" selected>one</option><option id=\"Jid.3\" value=\"2\">two</option></select>"
	if gotHtml := string(rq.Select(a, "disabled")); gotHtml != wantHtml {
		t.Errorf("Request.Select() = %q, want %q", gotHtml, wantHtml)
	}

	if !a.IsChecked("1") {
		t.Error("1 is not checked")
	}
	if a.IsChecked("2") {
		t.Error("2 is checked")
	}

	tmr := time.NewTimer(testTimeout)
	rq.inCh <- wsMsg{Data: "2", Jid: 1, What: what.Input}
	defer tmr.Stop()
	select {
	case <-tmr.C:
		t.Error("timeout")
	case <-a.setCalled:
	}

	if a.IsChecked("1") {
		t.Error("1 is checked")
	}
	if !a.IsChecked("2") {
		t.Error("2 is not checked")
	}

	select {
	case s := <-rq.outCh:
		t.Errorf("%q", s)
	default:
	}

	a.Set("2", false)
	rq.Dirty(a)

	select {
	case <-tmr.C:
		t.Error("timeout waiting for Value")
	case s := <-rq.outCh:
		if s != "Value\tJid.1\t\"\"\n" {
			t.Error("wrong Value")
		}
	}

	if a.IsChecked("1") {
		t.Error("1 is checked")
	}
	if a.IsChecked("2") {
		t.Error("2 is checked")
	}

	a.err = errors.New("meh")
	rq.inCh <- wsMsg{Data: "1", Jid: 1, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout waiting for Alert")
	case s := <-rq.outCh:
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}

	if a.IsChecked("1") {
		t.Error("1 is checked")
	}
	if a.IsChecked("2") {
		t.Error("2 is checked")
	}
}
