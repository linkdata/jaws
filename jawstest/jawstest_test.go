package jawstest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
)

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
