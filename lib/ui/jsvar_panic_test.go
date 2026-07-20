package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

var errPathSetterPanic = errors.New("path setter panic")

type panicSafePathState struct {
	Value string `json:"value"`
}

func (state *panicSafePathState) JawsSetPath(_ *jaws.Element, jsPath string, value any) (err error) {
	switch jsPath {
	case "":
		var next panicSafePathState
		var ok bool
		if next, ok = value.(panicSafePathState); !ok {
			return fmt.Errorf("unexpected root value type %T", value)
		}
		if *state == next {
			return jaws.ErrValueUnchanged
		}
		*state = next
	case "value":
		var next string
		var ok bool
		if next, ok = value.(string); !ok {
			return fmt.Errorf("unexpected value type %T", value)
		}
		if next == "panic" {
			panic(errPathSetterPanic)
		}
		if state.Value == next {
			return jaws.ErrValueUnchanged
		}
		state.Value = next
	default:
		return fmt.Errorf("unexpected path %q", jsPath)
	}
	return
}

func awaitJsVarOperation[T any](t *testing.T, operation string, ch <-chan T) (value T) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case value = <-ch:
	case <-timer.C:
		t.Fatalf("%s remained blocked", operation)
	}
	return
}

func TestJsVarPathSetterPanicReleasesValueLock(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	var mu sync.Mutex
	state := panicSafePathState{Value: "initial"}
	jsvar := NewJsVar(&mu, &state)
	type renderedJsVar struct {
		rq   *jaws.Request
		elem *jaws.Element
		err  error
	}
	renderedCh := make(chan renderedJsVar, 1)
	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		rendered := renderedJsVar{rq: jw.NewRequest(r)}
		rw := RequestWriter{Request: rendered.rq, Writer: w}
		if rendered.err = rw.HeadHTML(); rendered.err == nil {
			rendered.err = rw.JsVar("panicSafe", jsvar)
		}
		if rendered.err == nil {
			elems := rendered.rq.GetElements(&state)
			if len(elems) == 1 {
				rendered.elem = elems[0]
			} else {
				rendered.err = fmt.Errorf("rendered JsVar element count %d, want 1", len(elems))
			}
		}
		if rendered.err == nil {
			rendered.err = rw.TailHTML()
		}
		renderedCh <- rendered
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	httpResp, err := srv.Client().Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(io.Discard, httpResp.Body); err != nil {
		t.Fatal(err)
	}
	if err = httpResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("initial page status = %d, want %d", httpResp.StatusCode, http.StatusOK)
	}
	rendered := <-renderedCh
	if rendered.err != nil {
		t.Fatal(rendered.err)
	}

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/jaws/" + rendered.rq.JawsKeyString()
	header := http.Header{}
	header.Set("Origin", srv.URL)
	conn, wsResp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	if wsResp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("WebSocket status = %d, want %d", wsResp.StatusCode, http.StatusSwitchingProtocols)
	}

	// Send the exact what.Set frame that jaws.js emits for a browser-side write
	// over the production HTTP-upgraded WebSocket. Request event dispatch calls
	// CallEventHandlers, which recovers the panic and reports it to the browser
	// without terminating the request loop.
	incoming := wire.WsMsg{
		Jid:  rendered.elem.Jid(),
		What: what.Set,
		Data: `value="panic"`,
	}
	if err = conn.Write(ctx, websocket.MessageText, incoming.Append(nil)); err != nil {
		t.Fatal(err)
	}
	messageType, raw, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("panic response type = %v, want text", messageType)
	}
	alert, ok := wire.Parse(raw)
	if !ok || alert.What != what.Alert || !strings.Contains(alert.Data, errPathSetterPanic.Error()) {
		t.Fatalf("recovered PathSetter panic frame = %q, want Alert containing %q", raw, errPathSetterPanic)
	}

	getDone := make(chan panicSafePathState, 1)
	go func() {
		getDone <- jsvar.JawsGet(rendered.elem)
	}()
	if got := awaitJsVarOperation(t, "JawsGet", getDone); got != state {
		t.Fatalf("JawsGet after panic = %#v, want %#v", got, state)
	}

	want := panicSafePathState{Value: "recovered"}
	setDone := make(chan error, 1)
	go func() {
		setDone <- jsvar.JawsSet(rendered.elem, want)
	}()
	if err = awaitJsVarOperation(t, "JawsSet", setDone); err != nil {
		t.Fatal(err)
	}
	messageType, raw, err = conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("JawsSet response type = %v, want text", messageType)
	}
	wantSet := wire.WsMsg{Jid: rendered.elem.Jid(), What: what.Set, Data: `={"value":"recovered"}`}
	if !bytes.Equal(raw, wantSet.Append(nil)) {
		t.Fatalf("JawsSet frame after panic = %q, want %q", raw, wantSet.Append(nil))
	}
	if got := jsvar.JawsGet(rendered.elem); got != want {
		t.Fatalf("bound value after recovery = %#v, want %#v", got, want)
	}
}
