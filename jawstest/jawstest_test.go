package jawstest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// TestNewTestRequest_BcastChToOutCh drives a broadcast through the harness's
// exposed channels end to end: a page-global Alert injected on BcastCh must
// surface as the corresponding outbound frame on OutCh. This pins the channel
// wiring this package exposes (that BcastCh feeds the loop and OutCh carries its
// output, with the right directions), which the rest of the suite only exercises
// indirectly.
func TestNewTestRequest_BcastChToOutCh(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	// Alert is a page-global command: the loop emits exactly one Jid:0 frame
	// carrying Data verbatim regardless of Dest, so the expected OutCh frame is
	// deterministic.
	tr.BcastCh <- wire.Message{What: what.Alert, Data: "info\nhello"}

	select {
	case msg := <-tr.OutCh:
		if msg.What != what.Alert || msg.Jid != 0 || msg.Data != "info\nhello" {
			t.Errorf("OutCh = %+v, want {Jid:0 What:Alert Data:%q}", msg, "info\nhello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the broadcast to surface on OutCh")
	}
}

func TestNewTestRequest_SuccessAndClose(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	<-tr.ReadyCh

	if tr.Request == nil {
		t.Fatal("expected embedded request")
	}
	if tr.JawsKeyString() == "" {
		t.Fatal("expected a non-empty jaws key from the embedded request")
	}

	// The recorder starts empty; BodyString trims surrounding whitespace.
	if s := tr.BodyString(); s != "" {
		t.Errorf("BodyString = %q, want empty", s)
	}
	if h := tr.BodyHTML(); h != "" {
		t.Errorf("BodyHTML = %q, want empty", h)
	}
	tr.Recorder.Body.WriteString("  <b>hi</b>  ")
	if s := tr.BodyString(); s != "<b>hi</b>" {
		t.Errorf("BodyString = %q, want %q", s, "<b>hi</b>")
	}
	if h := tr.BodyHTML(); string(h) != "<b>hi</b>" {
		t.Errorf("BodyHTML = %q, want %q", h, "<b>hi</b>")
	}

	tr.Close()
	select {
	case <-tr.DoneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the loop to stop after Close")
	}
}

func TestNewTestRequest_WithExplicitRequest(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, httptest.NewRequest(http.MethodGet, "/explicit", nil))
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh
}

// TestClose_SecondCallPanics pins the documented contract that calling
// [jawstest.TestRequest.Close] more than once panics (it closes the already-closed
// InCh).
func TestClose_SecondCallPanics(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	<-tr.ReadyCh

	tr.Close()
	select {
	case <-tr.DoneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the loop to stop after Close")
	}

	defer func() {
		if recover() == nil {
			t.Error("expected a second Close to panic")
		}
	}()
	tr.Close()
}
