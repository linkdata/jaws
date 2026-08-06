package ui

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type containerAppendPanicChild struct {
	name    string
	err     error
	elem    *jaws.Element
	renders atomic.Int32
}

func (u *containerAppendPanicChild) JawsRender(elem *jaws.Element, w io.Writer, _ []any) (err error) {
	u.elem = elem
	u.renders.Add(1)
	if err = u.err; err == nil {
		b := elem.Jid().AppendStartTagAttr(nil, "span")
		b = append(b, '>')
		b = append(b, u.name...)
		b = append(b, "</span>"...)
		_, err = w.Write(b)
	}
	return
}

func (*containerAppendPanicChild) JawsUpdate(*jaws.Element) {}

type containerAppendPanickingContainer struct {
	container  Container
	panicValue any
	elem       *jaws.Element
}

func (u *containerAppendPanickingContainer) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	u.elem = elem
	if err = u.container.JawsRender(elem, w, params); err == nil {
		panic(u.panicValue)
	}
	return
}

func (u *containerAppendPanickingContainer) JawsUpdate(elem *jaws.Element) {
	u.container.JawsUpdate(elem)
}

func containerAppendPanicValue(fn func()) (panicValue any) {
	defer func() { panicValue = recover() }()
	fn()
	return
}

func containerAppendPanicMessages(t *testing.T, tr *jawstest.TestRequest) (messages []wire.WsMsg) {
	t.Helper()
	tr.InCh <- wire.WsMsg{}
	probe := "container append panic: " + t.Name()
	tr.BcastCh <- wire.Message{What: what.Alert, Data: probe}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		select {
		case msg, ok := <-tr.OutCh:
			if !ok {
				t.Fatal("request loop stopped before the append-panic probe arrived")
			}
			if msg.What == what.Alert && msg.Data == probe {
				return
			}
			messages = append(messages, msg)
		case <-tr.DoneCh:
			t.Fatal("request loop stopped before the append-panic probe arrived")
		case <-ctx.Done():
			t.Fatal("timed out waiting for the append-panic probe")
		}
	}
}

func TestContainerAppendErrorPanicCleansPendingTail(t *testing.T) {
	tr := newReuseRequest(t) // No Logger: MustLog panics with the render error.
	provider := new(testContainer)
	container := NewContainer("div", provider)
	elem := tr.NewElement(container)
	if err := elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}

	renderErr := errors.New("append render failed")
	first := &containerAppendPanicChild{name: "first"}
	failing := &containerAppendPanicChild{name: "failing", err: renderErr}
	tail := &containerAppendPanicChild{name: "tail"}
	provider.contents = []jaws.UI{first, failing, tail}

	if panicValue := containerAppendPanicValue(elem.JawsUpdate); panicValue != renderErr {
		t.Fatalf("panic = %v, want original render error %v", panicValue, renderErr)
	}
	contents := containerElements(t, elem)
	if len(contents) != 1 || contents[0].UI() != first {
		t.Fatalf("contents after panic = %v, want only successfully appended prefix", contents)
	}
	if first.elem == nil || first.elem.Deleted() {
		t.Fatal("successfully appended child was not preserved")
	}
	if failing.elem == nil || !failing.elem.Deleted() {
		t.Fatal("returned-error child remains live")
	}
	if got := tail.renders.Load(); got != 0 {
		t.Fatalf("pending tail render calls = %d, want 0", got)
	}
	if got := countRegistered(t, tr.Request); got != 2 {
		t.Fatalf("registered Elements after panic = %d, want parent and successful child", got)
	}

	var appends int
	for _, msg := range containerAppendPanicMessages(t, tr) {
		switch msg.What {
		case what.Append:
			appends++
			if msg.Jid != elem.Jid() || !strings.Contains(msg.Data, first.elem.Jid().String()) {
				t.Fatalf("preserved Append = %+v, want parent %v and child %v", msg, elem.Jid(), first.elem.Jid())
			}
		case what.Remove, what.Order:
			t.Fatalf("panic queued unexpected browser operation %+v", msg)
		}
	}
	if appends != 1 {
		t.Fatalf("Append messages = %d, want the successful prefix only", appends)
	}
}

func TestContainerAppendRenderPanicCleansCurrentAndPendingTail(t *testing.T) {
	tr := newReuseRequest(t)
	provider := new(testContainer)
	container := NewContainer("div", provider)
	elem := tr.NewElement(container)
	if err := elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}

	nested := &containerAppendPanicChild{name: "nested"}
	panicValue := errors.New("child render panic")
	panicking := &containerAppendPanickingContainer{
		container:  NewContainer("section", &testContainer{contents: []jaws.UI{nested}}),
		panicValue: panicValue,
	}
	tail := &containerAppendPanicChild{name: "tail"}
	provider.contents = []jaws.UI{panicking, tail}

	if got := containerAppendPanicValue(elem.JawsUpdate); got != panicValue {
		t.Fatalf("panic = %v, want original panic value %v", got, panicValue)
	}
	if contents := containerElements(t, elem); len(contents) != 0 {
		t.Fatalf("outer contents after panic = %v, want none", contents)
	}
	if panicking.elem == nil || !panicking.elem.Deleted() {
		t.Fatal("panicking child remains live")
	}
	if contents := containerElements(t, panicking.elem); len(contents) != 0 {
		t.Fatalf("panicking child's owned contents = %v, want detached cleanup", contents)
	}
	if nested.elem == nil || !nested.elem.Deleted() {
		t.Fatal("nested child owned by the panicking child remains live")
	}
	if got := tail.renders.Load(); got != 0 {
		t.Fatalf("pending tail render calls = %d, want 0", got)
	}
	if got := countRegistered(t, tr.Request); got != 1 {
		t.Fatalf("registered Elements after panic = %d, want the parent only", got)
	}
	for _, msg := range containerAppendPanicMessages(t, tr) {
		if msg.What == what.Append || msg.What == what.Remove || msg.What == what.Order {
			t.Fatalf("panicking append queued browser operation %+v", msg)
		}
	}
}
