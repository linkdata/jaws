package jaws

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUseRequestDoesNotRelogCancelledPendingRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	jw.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	const marker = "cancelled-request-marker"
	initial := httptest.NewRequest("GET", "/?"+marker, nil)
	rq := jw.NewRequest(httptest.NewRecorder(), initial)
	rq.Cancel(errors.New("render failed"))
	for range 4 {
		claim := httptest.NewRequest("GET", "/jaws/"+rq.JawsKeyString(), nil)
		if got := jw.UseRequest(rq.JawsKey, claim); got != nil {
			t.Fatal("cancelled Request was claimed")
		}
	}
	jw.Close()
	<-jw.loggerQueue.doneCh
	if count := strings.Count(logs.String(), marker); count != 1 {
		t.Fatalf("initial URI logged %d times, want once: %s", count, logs.String())
	}
}
