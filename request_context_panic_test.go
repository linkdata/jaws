package jaws

import (
	"context"
	"errors"
	"testing"
	"time"
)

type panicErrContext struct{ context.Context }

func (panicErrContext) Err() error { panic("context Err") }

type panicStopContext struct {
	context.Context
	done chan struct{}
}

func (ctx panicStopContext) Done() <-chan struct{} { return ctx.done }
func (panicStopContext) AfterFunc(func()) func() bool {
	return func() bool { panic("context AfterFunc stop") }
}

func recoverContextPanic(fn func()) (value any) {
	defer func() { value = recover() }()
	fn()
	return
}

func assertContextLocksReleased(t *testing.T, jw *Jaws, rq *Request) {
	t.Helper()
	if !jw.mu.TryLock() {
		t.Fatal("context panic left Jaws locked")
	}
	jw.mu.Unlock()
	if !rq.mu.TryLock() {
		t.Fatal("context panic left Request locked")
	}
	rq.mu.Unlock()
}

func TestContextErrPanicReleasesRequestLifecycleLocks(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*Jaws, *Request)
		run     func(*Jaws, *Request)
	}{
		{name: "UseRequest", run: func(jw *Jaws, rq *Request) { jw.UseRequest(rq.JawsKey, nil) }},
		{name: "startServe", prepare: func(jw *Jaws, rq *Request) { jw.UseRequest(rq.JawsKey, nil) }, run: func(_ *Jaws, rq *Request) { rq.startServe() }},
		{name: "Cancel", run: func(_ *Jaws, rq *Request) { rq.Cancel(nil) }},
		{name: "cancelIfCurrent", run: func(jw *Jaws, rq *Request) { jw.cancelIfCurrent(rq.JawsKey, rq, errors.New("write failed")) }},
		{name: "pending eviction", prepare: func(jw *Jaws, _ *Request) { jw.MaxPendingRequestsPerIP = 1 }, run: func(jw *Jaws, _ *Request) { jw.newRequest(nil) }},
		{name: "maintenance", run: func(jw *Jaws, _ *Request) { jw.maintenance(time.Minute) }},
		{name: "recycle", run: func(jw *Jaws, rq *Request) { jw.recycle(rq) }},
		{name: "Close", run: func(jw *Jaws, _ *Request) { jw.Close() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			rq := jw.newRequest(nil)
			if tt.prepare != nil {
				tt.prepare(jw, rq)
			}
			original := rq.Context()
			rq.SetContext(func(context.Context) context.Context { return panicErrContext{original} })
			if got := recoverContextPanic(func() { tt.run(jw, rq) }); got != "context Err" {
				t.Fatalf("panic = %v, want context Err", got)
			}
			assertContextLocksReleased(t, jw, rq)
			rq.SetContext(func(context.Context) context.Context { return original })
			jw.recycle(rq)
			jw.Close()
		})
	}
}

func TestCloseRunningRequestAfterFuncStopPanicReleasesLocks(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.BaseContext = panicStopContext{Context: context.Background(), done: make(chan struct{})}
	rq := jw.newRequest(nil)
	if got := jw.UseRequest(rq.JawsKey, nil); got != rq {
		t.Fatalf("UseRequest = %p, want %p", got, rq)
	}
	if !rq.startServe() {
		t.Fatal("startServe failed")
	}
	if got := recoverContextPanic(jw.Close); got != "context AfterFunc stop" {
		t.Fatalf("panic = %v, want context AfterFunc stop", got)
	}
	assertContextLocksReleased(t, jw, rq)
	jw.recycle(rq)
}
