package ui

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
)

type selectRenderPhaseHandler struct {
	containsCalls atomic.Int32
	getCalls      atomic.Int32

	getterStarted  chan struct{}
	releaseGetter  chan struct{}
	unexpected     chan struct{}
	startedOnce    sync.Once
	releaseOnce    sync.Once
	unexpectedOnce sync.Once
}

func newSelectRenderPhaseHandler() *selectRenderPhaseHandler {
	return &selectRenderPhaseHandler{
		getterStarted: make(chan struct{}),
		releaseGetter: make(chan struct{}),
		unexpected:    make(chan struct{}),
	}
}

func (h *selectRenderPhaseHandler) reportCallback(calls int32) {
	if calls > 1 {
		h.unexpectedOnce.Do(func() { close(h.unexpected) })
	}
}

func (h *selectRenderPhaseHandler) JawsContains(*jaws.Element) []jaws.UI {
	h.reportCallback(h.containsCalls.Add(1))
	return nil
}

func (h *selectRenderPhaseHandler) JawsGet(*jaws.Element) string {
	h.reportCallback(h.getCalls.Add(1))
	h.startedOnce.Do(func() { close(h.getterStarted) })
	<-h.releaseGetter
	return ""
}

func (*selectRenderPhaseHandler) JawsSet(*jaws.Element, string) error { return nil }

func (h *selectRenderPhaseHandler) release() {
	h.releaseOnce.Do(func() { close(h.releaseGetter) })
}

type selectRenderPhaseLogger struct {
	log testErrorLog
}

func (*selectRenderPhaseLogger) Info(string, ...any) {}
func (*selectRenderPhaseLogger) Warn(string, ...any) {}
func (l *selectRenderPhaseLogger) Error(_ string, args ...any) {
	l.log.record(args)
}

func (l *selectRenderPhaseLogger) sync(t *testing.T, jw *jaws.Jaws) []error {
	return l.log.sync(t, jw)
}

func TestSelectRenderPhaseIncludesInitialValue(t *testing.T) {
	logger := new(selectRenderPhaseLogger)
	jw, rq := newConfiguredCoreRequest(t, func(jw *jaws.Jaws) { jw.Logger = logger })
	handler := newSelectRenderPhaseHandler()
	defer handler.release()

	ui := NewSelect(handler)
	elem := rq.NewElement(ui)
	renderDone := make(chan error, 1)
	go func() {
		renderDone <- elem.JawsRender(io.Discard, nil)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-handler.getterStarted:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the initial Select getter")
	}

	st := requireContainerState(t, elem)
	st.mu.Lock()
	rendering := st.rendering
	st.mu.Unlock()
	if !rendering {
		t.Fatal("Select state left the rendering phase before its initial value getter completed")
	}

	updateDone := make(chan struct{})
	go func() {
		elem.JawsUpdate()
		close(updateDone)
	}()
	var contentionFailure string
	select {
	case <-handler.unexpected:
		contentionFailure = "contending Select update invoked the handler"
	case <-updateDone:
	case <-ctx.Done():
		contentionFailure = "timed out waiting for the contending Select update"
	}
	handler.release()
	select {
	case <-updateDone:
	case <-ctx.Done():
		t.Fatal("timed out finishing the contending Select update")
	}
	select {
	case err := <-renderDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for Select render completion")
	}
	if contentionFailure != "" {
		t.Fatal(contentionFailure)
	}
	if got := handler.containsCalls.Load(); got != 1 {
		t.Fatalf("JawsContains calls during contention = %d, want 1 initial-render call", got)
	}
	if got := handler.getCalls.Load(); got != 1 {
		t.Fatalf("JawsGet calls during contention = %d, want 1 blocked initial-render call", got)
	}
	logged := logger.sync(t, jw)
	if len(logged) != 1 || !errors.Is(logged[0], jaws.ErrElementStateClaimed) {
		t.Fatalf("logged errors = %v, want one %v", logged, jaws.ErrElementStateClaimed)
	}
}
