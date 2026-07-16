package jaws

import (
	"errors"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/what"
)

func TestJawsJsCallReachesRequestTargets(t *testing.T) {
	tj := newTestJaws()
	defer tj.Close()

	requests := []*testRequest{
		newWrappedTestRequest(tj.Jaws, nil),
		newWrappedTestRequest(tj.Jaws, nil),
	}
	defer func() {
		for _, tr := range requests {
			tr.Close()
		}
		for _, tr := range requests {
			<-tr.DoneCh
		}
	}()
	for _, tr := range requests {
		<-tr.ReadyCh
	}

	assertCall := func(tr *testRequest, wantData string) {
		t.Helper()
		select {
		case msg := <-tr.OutCh:
			if msg.What != what.Call {
				t.Fatalf("message type = %v, want %v", msg.What, what.Call)
			}
			if msg.Jid != 0 {
				t.Fatalf("message Jid = %v, want whole-request Jid 0", msg.Jid)
			}
			if msg.Data != wantData {
				t.Fatalf("message data = %q, want %q", msg.Data, wantData)
			}
		case <-time.After(time.Second):
			t.Fatal("Jaws.JsCall did not reach the selected request")
		}
	}

	tj.JsCall(nil, "app.refresh", `{"scope":"all"}`)
	for _, tr := range requests {
		assertCall(tr, `app.refresh={"scope":"all"}`)
	}

	tj.JsCall(requests[0].JawsKey, "app.refresh", `{"scope":"one"}`)
	assertCall(requests[0], `app.refresh={"scope":"one"}`)
	tj.JsCall(requests[1].JawsKey, "app.refresh", `{"scope":"other"}`)
	assertCall(requests[1], `app.refresh={"scope":"other"}`)

	// A page-global message is a per-subscription barrier: if either key-targeted
	// Call leaked to the other Request, it appears before Reload and fails here.
	tj.Reload()
	for _, tr := range requests {
		select {
		case msg := <-tr.OutCh:
			if msg.What != what.Reload {
				t.Fatalf("request-key Call reached an unselected request before barrier: %+v", msg)
			}
		case <-time.After(time.Second):
			t.Fatal("Reload barrier did not reach request")
		}
	}
}

func TestJawsJsCallRejectsEmptyHTMLIDTarget(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger

	call := func() {
		jw.JsCall("", "app.refresh", `{}`)
	}
	if deadlock.Debug {
		func() {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("empty HTML-id Call did not panic under deadlock.Debug")
				}
			}()
			call()
		}()
	} else {
		call()
	}

	if !errors.Is(logger.err, ErrEmptyCallTarget) {
		t.Fatalf("empty HTML-id Call error = %v", logger.err)
	}
	if got := len(jw.bcastCh); got != 0 {
		t.Fatalf("empty HTML-id Call queued %d broadcasts, want none", got)
	}
}
