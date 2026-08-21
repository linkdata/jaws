package jaws

import (
	"net/http/httptest"
	"testing"
)

func TestNewRequest_SetsCacheControlBeforeHeadHTML(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()
	waitForServeLoop(t, jw)

	w := httptest.NewRecorder()
	w.Header().Add("Cache-Control", "public")
	w.Header().Add("Cache-Control", "max-age=3600")
	rq := jw.NewRequest(w, nil)
	if _, err := w.Write([]byte("<!doctype html>")); err != nil {
		t.Fatal(err)
	}
	if err := rq.HeadHTML(w); err != nil {
		t.Fatal(err)
	}
	if got := w.Result().Header.Values("Cache-Control"); len(got) != 1 || got[0] != "no-store" {
		t.Fatalf("Cache-Control = %q, want [%q]", got, "no-store")
	}
}
