package jaws_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type panickingUpdater struct{ value any }

func (panickingUpdater) JawsRender(*jaws.Element, io.Writer, []any) error { return nil }
func (u panickingUpdater) JawsUpdate(*jaws.Element)                       { panic(u.value) }

func TestTestServeReportsUpdaterPanic(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(r)
	wantPanic := errors.New("update boom")
	elem := rq.NewElement(panickingUpdater{value: wantPanic})
	const updateTag = tag.Tag("panic-update")
	elem.Tag(updateTag)
	elem.Freeze()
	if got := jw.UseRequest(rq.JawsKey, r); got != rq {
		t.Fatal("could not claim request")
	}

	panicCh := make(chan any, 1)
	_, _, bcastCh, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		panicCh <- recovered
	})
	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for TestServe to start")
	}
	bcastCh <- wire.Message{Dest: updateTag, What: what.Update}
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for TestServe to stop")
	}

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
