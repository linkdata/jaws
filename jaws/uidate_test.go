package jaws

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func TestRequest_Date(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter(time.Now())
	want := fmt.Sprintf(`<input id="Jid.1" type="date" value="%s">`, ts.Get().Format(ISO8601))
	rq.Date(ts)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Date() = %q, want %q", got, want)
	}

	val, _ := time.Parse(ISO8601, "1970-02-03")
	rq.inCh <- wsMsg{Data: val.Format(ISO8601), Jid: 1, What: what.Input}
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	select {
	case <-th.C:
		th.Timeout()
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

	val = time.Now()
	ts.Set(val)
	rq.Dirty(ts)
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != fmt.Sprintf("Value\tJid.1\t\"%s\"\n", val.Format(ISO8601)) {
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
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Alert\t\t\"danger\\nparsing time &#34;omg&#34; as &#34;2006-01-02&#34;: cannot parse &#34;omg&#34; as &#34;2006&#34;\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}

	ts.err = errors.New("meh")
	rq.inCh <- wsMsg{Data: val.Format(ISO8601), Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Alert\t\t\"danger\\nmeh\"\n" {
			t.Errorf("wrong Alert: %q", s)
		}
	}
}
