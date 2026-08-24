package jaws_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestJawsSetupIgnoresNilSetupFunc(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	var nilSetup jaws.SetupFunc
	if err = jw.Setup(nil, "", nilSetup); err != nil {
		t.Fatalf("Setup(nil SetupFunc) error = %v, want nil", err)
	}

	firstErr := errors.New("first setup error")
	secondErr := errors.New("second setup error")
	var calls int
	first := jaws.SetupFunc(func(_ *jaws.Jaws, _ jaws.HandleFunc, _ string) (urls []*url.URL, err error) {
		calls++
		err = firstErr
		return
	})
	second := jaws.SetupFunc(func(_ *jaws.Jaws, _ jaws.HandleFunc, _ string) (urls []*url.URL, err error) {
		calls++
		err = secondErr
		return
	})
	err = jw.Setup(nil, "", first, nilSetup, second)
	if calls != 2 {
		t.Fatalf("Setup called %d non-nil setup functions, want 2", calls)
	}
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("Setup error = %v, want both setup errors", err)
	}
}

type panickingUpdater struct{ value any }

func (panickingUpdater) JawsRender(*jaws.Element, io.Writer, []any) error { return nil }
func (u panickingUpdater) JawsUpdate(*jaws.Element)                       { panic(u.value) }

func startPanickingTestServe(t *testing.T, onPanic func(recovered any)) (jw *jaws.Jaws, doneCh chan struct{}, wantPanic error) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(httptest.NewRecorder(), r)
	wantPanic = errors.New("update boom")
	elem := rq.NewElement(panickingUpdater{value: wantPanic})
	const updateTag = tag.Tag("panic-update")
	elem.Tag(updateTag)
	elem.Freeze()
	if got := jw.UseRequest(rq.JawsKey, r); got != rq {
		t.Fatal("could not claim request")
	}

	_, _, bcastCh, readyCh, doneCh := jw.TestServe(rq, onPanic)
	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for TestServe to start")
	}
	bcastCh <- wire.Message{Dest: updateTag, What: what.Update}
	return
}

func waitForTestServeDone(t *testing.T, doneCh <-chan struct{}) {
	t.Helper()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for TestServe to stop")
	}
}

func TestTestServeReportsUpdaterPanic(t *testing.T) {
	panicCh := make(chan any, 1)
	jw, doneCh, wantPanic := startPanickingTestServe(t, func(recovered any) {
		panicCh <- recovered
	})
	waitForTestServeDone(t, doneCh)

	select {
	case recovered := <-panicCh:
		if recovered != wantPanic {
			t.Fatalf("onPanic recovered %#v, want original value %#v", recovered, wantPanic)
		}
	default:
		t.Fatal("onPanic was not called")
	}
	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("RequestCount() = %d, want 0 after TestServe recycled the request", got)
	}
}

func TestTestServeClosesDoneWhenOnPanicGoexits(t *testing.T) {
	panicCh := make(chan any, 1)
	jw, doneCh, wantPanic := startPanickingTestServe(t, func(recovered any) {
		panicCh <- recovered
		runtime.Goexit()
	})
	waitForTestServeDone(t, doneCh)
	select {
	case recovered := <-panicCh:
		if recovered != wantPanic {
			t.Fatalf("onPanic recovered %#v, want original value %#v", recovered, wantPanic)
		}
	default:
		t.Fatal("onPanic was not called")
	}
	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("RequestCount() = %d, want 0 after TestServe recycled the request", got)
	}
}
