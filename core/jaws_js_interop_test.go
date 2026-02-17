package core

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func nextOutboundMsg(t *testing.T, rq *testRequest) WsMsg {
	t.Helper()
	select {
	case msg := <-rq.OutCh:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for outbound ws message")
		return WsMsg{}
	}
}

func TestRequest_IncomingRemoveDoesNotDeleteMessageJid(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})

	select {
	case rq.InCh <- WsMsg{What: what.Remove, Jid: elem.Jid(), Data: ""}:
	case <-time.After(time.Second):
		t.Fatal("timeout sending incoming Remove message")
	}

	select {
	case <-time.After(20 * time.Millisecond):
	case <-rq.DoneCh:
		t.Fatal("request shut down unexpectedly")
	}
	if got := rq.GetElementByJid(elem.Jid()); got == nil {
		t.Fatalf("element %s should still exist after Remove with empty data", elem.Jid())
	}
}

func TestRequest_ReplaceMessageTargetsElementHTML(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tag := &testUi{}
	jid := rq.Register(tag)
	html := `<div id="` + jid.String() + `">replaced</div>`

	rq.Jaws.Replace(tag, html)
	msg := nextOutboundMsg(t, rq)

	if msg.What != what.Replace {
		t.Fatalf("unexpected message type %v", msg.What)
	}
	if msg.Data != html {
		t.Fatalf("replace payload mismatch: got %q want %q", msg.Data, html)
	}
}

func TestElement_ReplaceMessageTargetsElementHTML(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tag := &testUi{}
	jid := rq.Register(tag)
	elem := rq.GetElementByJid(jid)
	if elem == nil {
		t.Fatal("missing element")
	}
	html := `<div id="` + jid.String() + `">replaced</div>`

	elem.Replace(template.HTML(html))
	msg := nextOutboundMsg(t, rq)

	if msg.What != what.Replace {
		t.Fatalf("unexpected message type %v", msg.What)
	}
	if msg.Data != html {
		t.Fatalf("replace payload mismatch: got %q want %q", msg.Data, html)
	}
}

func TestRequest_JsCallProducesJawsJSFrameSafeWireData(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tag := &testUi{}
	rq.Register(tag)

	tests := []struct {
		name    string
		jsonstr string
	}{
		{
			name:    "pretty json with newline",
			jsonstr: "{\n\"a\":1}",
		},
		{
			name:    "pretty json with tab",
			jsonstr: "{\t\"a\":1}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq.Jaws.JsCall(tag, "fn", tt.jsonstr)
			msg := nextOutboundMsg(t, rq)
			wire := msg.Format()

			if got := strings.Count(wire, "\n"); got != 1 {
				t.Fatalf("wire message contains embedded newlines (%d): %q", got, wire)
			}
			if got := strings.Count(wire, "\t"); got != 2 {
				t.Fatalf("wire message contains embedded tab separators (%d): %q", got, wire)
			}
		})
	}
}

func TestRequest_JsCallFunctionPathDoesNotBreakWireFraming(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tag := &testUi{}
	rq.Register(tag)

	tests := []struct {
		name   string
		jsfunc string
	}{
		{
			name:   "tab in function path",
			jsfunc: "fn\tpart",
		},
		{
			name:   "newline in function path",
			jsfunc: "fn\npart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq.Jaws.JsCall(tag, tt.jsfunc, `{"a":1}`)
			msg := nextOutboundMsg(t, rq)
			wire := msg.Format()

			if got := strings.Count(wire, "\n"); got != 1 {
				t.Fatalf("wire message contains embedded newlines (%d): %q", got, wire)
			}
			if got := strings.Count(wire, "\t"); got != 2 {
				t.Fatalf("wire message contains embedded tab separators (%d): %q", got, wire)
			}
		})
	}
}

func TestRequest_IncomingRemoveWithZeroContainerJidIsIgnored(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})

	select {
	case rq.InCh <- WsMsg{What: what.Remove, Jid: 0, Data: elem.Jid().String()}:
	case <-time.After(time.Second):
		t.Fatal("timeout sending incoming Remove message")
	}

	select {
	case <-time.After(20 * time.Millisecond):
	case <-rq.DoneCh:
		t.Fatal("request shut down unexpectedly")
	}
	if got := rq.GetElementByJid(elem.Jid()); got == nil {
		t.Fatalf("element %s should not be deletable through zero-container Remove", elem.Jid())
	}
}
