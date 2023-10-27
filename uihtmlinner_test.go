package jaws

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestUiHtmlInner_JawsUpdate(t *testing.T) {
	jw := New()
	defer jw.Close()
	nextJid = 0
	ts := newTestSetter(template.HTML("first"))
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	ui := NewUiDiv(ts)
	elem := rq.NewElement(ui)
	var sb strings.Builder
	if err := ui.JawsRender(elem, &sb, nil); err != nil {
		t.Fatal(err)
	}
	wantHtml := "<div id=\"Jid.1\">first</div>"
	if sb.String() != wantHtml {
		t.Errorf("got %q, want %q", sb.String(), wantHtml)
	}
	ts.Set(template.HTML("second"))
	ui.JawsUpdate(elem)
	want := []wsMsg{{
		Data: "second",
		Jid:  1,
		What: what.Inner,
	}}
	if !slices.Equal(elem.wsQueue, want) {
		t.Errorf("got %v, want %v", elem.wsQueue, want)
	}
}
