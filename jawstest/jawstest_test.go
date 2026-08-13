package jawstest_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

const jawstestUpdaterPanic = "jawstest updater panic sentinel"

type jawstestPanickingUpdater struct{}

func (jawstestPanickingUpdater) JawsRender(*jaws.Element, io.Writer, []any) error { return nil }
func (jawstestPanickingUpdater) JawsUpdate(*jaws.Element)                         { panic(jawstestUpdaterPanic) }

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

func TestNewTestRequest_ReportsUpdaterPanic(t *testing.T) {
	const helperEnv = "JAWS_TEST_UPDATER_PANIC_HELPER"
	if os.Getenv(helperEnv) == "1" {
		jw, err := jaws.New()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(jw.Close)
		go jw.Serve()

		tr := jawstest.NewTestRequest(jw, nil)
		select {
		case <-tr.ReadyCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for the request loop to start")
		}
		const updateTag = tag.Tag("jawstest-panic-update")
		elem := tr.NewElement(jawstestPanickingUpdater{})
		elem.Tag(updateTag)
		elem.Freeze()
		tr.BcastCh <- wire.Message{Dest: updateTag, What: what.Update}
		select {
		case <-tr.DoneCh:
			t.Fatal("request loop returned normally after JawsUpdate panic")
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for JawsUpdate panic")
		}
		return
	}

	// NewTestRequest re-panics on its request-loop goroutine, so a subprocess is
	// required to observe the panic without terminating this test process.
	// #nosec G204 -- os.Args[0] is the current test binary and all arguments are fixed.
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestNewTestRequest_ReportsUpdaterPanic$", "-test.count=1", "-test.timeout=5s")
	cmd.Env = append(os.Environ(), helperEnv+"=1", "GOTRACEBACK=none")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("jawstest request loop returned normally after JawsUpdate panic")
	}
	if !strings.Contains(string(output), "panic: "+jawstestUpdaterPanic) {
		t.Fatalf("subprocess did not report the updater panic:\n%s", output)
	}
}

// TestClose_SecondCallIsSafe pins that [jawstest.TestRequest.Close] is idempotent:
// a second call is a no-op rather than panicking on the already-closed InCh.
func TestClose_SecondCallIsSafe(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	<-tr.ReadyCh

	tr.Close()
	select {
	case <-tr.DoneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for the loop to stop after Close")
	}

	// A second Close must not panic on the already-closed InCh.
	tr.Close()
}
