package ui

import (
	"errors"
	"html/template"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type jsVarRollbackEventGate struct {
	blocked   chan struct{}
	release   chan struct{}
	processed chan struct{}
}

func (gate *jsVarRollbackEventGate) JawsRender(elem *jaws.Element, w io.Writer, _ []any) error {
	elem.Tag(gate)
	b := elem.Jid().AppendStartTagAttr(nil, "div")
	_, err := w.Write(append(b, "></div>"...))
	return err
}

func (*jsVarRollbackEventGate) JawsUpdate(*jaws.Element) {}

func (gate *jsVarRollbackEventGate) JawsInput(_ *jaws.Element, input string) error {
	switch input {
	case "block":
		close(gate.blocked)
		<-gate.release
	case "mark":
		close(gate.processed)
	}
	return nil
}

type jsVarRollbackRuntimeDot struct {
	Store   *JsVarStore[int]
	fail    bool
	entered chan struct{}
	release chan struct{}
}

func (dot *jsVarRollbackRuntimeDot) Check() (string, error) {
	if dot.fail {
		close(dot.entered)
		<-dot.release
		return "", errJsVarLifecycleRender
	}
	return "", nil
}

func waitJsVarRollbackSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

// TestJsVarBindingRollbackProcessesOldRouteProposal drives the real Request
// loop's event FIFO while a failed Template update holds a provisional route.
func TestJsVarBindingRollbackProcessesOldRouteProposal(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	logger := new(templateLogger)
	jw.Logger = logger
	if err = jw.AddTemplateLookuper(template.Must(template.New("rollback-runtime").Parse(
		`{{define "rollback-runtime"}}{{$.NewUI ($.Dot.Store.Bind)}}{{$.Dot.Check}}{{end}}`,
	))); err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	t.Cleanup(jw.Close)
	tr := jawstest.NewTestRequest(jw, nil)
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	waitJsVarRollbackSignal(t, tr.ReadyCh, "Request readiness")

	var mu sync.RWMutex
	value := 1
	store := newTestJsVarStore(t, jw, "client", &mu, &value)
	store.ClientCheck = func(*jaws.Element, *int, string) error { return nil }
	dot := &jsVarRollbackRuntimeDot{
		Store:   store,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	gate := &jsVarRollbackEventGate{
		blocked:   make(chan struct{}),
		release:   make(chan struct{}),
		processed: make(chan struct{}),
	}
	t.Cleanup(func() {
		select {
		case <-dot.release:
		default:
			close(dot.release)
		}
		select {
		case <-gate.release:
		default:
			close(gate.release)
		}
	})
	rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
	if err = rw.NewUI(NewTemplate("div", "rollback-runtime", dot)); err != nil {
		t.Fatal(err)
	}
	if err = rw.NewUI(gate); err != nil {
		t.Fatal(err)
	}
	parents := tr.GetElements(dot)
	bindings := tr.GetElements(store)
	if len(parents) != 1 || len(bindings) != 1 {
		t.Fatalf("initial parent/binding count = %d/%d, want 1/1", len(parents), len(bindings))
	}
	old := bindings[0]
	gateElem := tr.GetElements(gate)[0]

	// Keep eventCaller busy while the Request loop accepts an old-route proposal.
	tr.InCh <- wire.WsMsg{Jid: gateElem.Jid(), What: what.Input, Data: "block"}
	waitJsVarRollbackSignal(t, gate.blocked, "event gate")
	tr.InCh <- wire.WsMsg{Jid: old.Jid(), What: what.Proposal, Data: "=2"}
	tr.InCh <- wire.WsMsg{Jid: gateElem.Jid(), What: what.Input, Data: "mark"}

	// The new binding is rendered into a private buffer, then Check holds the
	// update before it can replace the old browser DOM or roll back.
	dot.fail = true
	tr.BcastCh <- wire.Message{Dest: dot, What: what.Update}
	waitJsVarRollbackSignal(t, dot.entered, "provisional route")
	if got := len(tr.GetElements(store)); got != 2 {
		t.Fatalf("bindings during tentative render = %d, want 2", got)
	}
	close(gate.release)
	waitJsVarRollbackSignal(t, gate.processed, "old-route proposal")
	mu.RLock()
	gotValue := value
	mu.RUnlock()
	if gotValue != 2 {
		t.Fatalf("old-route proposal during tentative render left value = %d, want 2", gotValue)
	}
	close(dot.release)
	for {
		select {
		case msg := <-tr.OutCh:
			if msg.What == what.Patch {
				if msg.Jid != old.Jid() || msg.Data != "=2" {
					t.Fatalf("rollback patch = %+v, want old route root value 2", msg)
				}
				if old.Deleted() {
					t.Fatal("rollback deleted the original route")
				}
				if got := tr.GetElements(store); len(got) != 1 || got[0] != old {
					t.Fatalf("bindings after rollback = %v, want only original", got)
				}
				if logged := logger.sync(t, jw); len(logged) != 1 || !errors.Is(logged[0], errJsVarLifecycleRender) {
					t.Fatalf("render errors = %v, want rollback error", logged)
				}
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for canonical patch on restored route")
		}
	}
}
