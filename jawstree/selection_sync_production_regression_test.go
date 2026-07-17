package jawstree

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestTree_SelectionSyncClosesRenderToConnectGap(t *testing.T) {
	jw, err := jaws.New()
	maybeError(t, err)
	defer jw.Close()
	go jw.Serve()

	active := jawstest.NewTestRequest(jw, nil)
	defer func() {
		active.Close()
		<-active.DoneCh
	}()
	<-active.ReadyCh

	root := &Node{Children: []*Node{{Name: "a"}, {Name: "b"}}}
	var mu sync.RWMutex
	tree := New(ui.NewJsVar(&mu, root))
	activeElem := active.NewElement(tree)
	maybeError(t, activeElem.JawsRender(&strings.Builder{}, nil))
	active.InCh <- wire.WsMsg{}
	select {
	case msg := <-active.OutCh:
		if msg.What != what.Call || !strings.HasPrefix(msg.Data, "jawstreeInit=") {
			t.Fatalf("active initializer = %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the active initializer")
	}

	initial := httptest.NewRequest("GET", "/", nil)
	pending := jw.NewRequest(initial)
	pendingElem := pending.NewElement(tree)
	callerInput := make(chan string, 1)
	catchAllInput := jaws.InputFn(func(_ *jaws.Element, value string) (err error) {
		callerInput <- value
		return
	})
	maybeError(t, pendingElem.JawsRender(&strings.Builder{}, []any{catchAllInput}))

	// Move the shared tree after the second page has rendered but before that
	// Request subscribes. Broadcasts correctly target active Requests only, so
	// the pending page cannot observe this selection through the ordinary path.
	active.InCh <- wire.WsMsg{Jid: activeElem.Jid(), What: what.Set, Data: "children.1.selected=true"}
	for {
		select {
		case msg := <-active.OutCh:
			if strings.HasPrefix(msg.Data, "jawstreeSetSelection=") {
				goto selectionDistributed
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for the active selection broadcast")
		}
	}

selectionDistributed:
	if selected := tree.GetSelected(); !reflect.DeepEqual(selected, [][]string{{"b"}}) {
		t.Fatalf("server selection = %v, want b", selected)
	}

	inCh, outCh, _, readyCh, doneCh := jw.TestServe(pending, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		close(inCh)
		<-doneCh
	}()
	<-readyCh

	select {
	case msg := <-outCh:
		payload, ok := strings.CutPrefix(msg.Data, "jawstreeInit=")
		if msg.What != what.Call || msg.Jid != pendingElem.Jid() || !ok {
			t.Fatalf("pending initializer = %+v", msg)
		}
		var init struct {
			SelectionVersion uint64 `json:"selectionVersion"`
		}
		if err = json.Unmarshal([]byte(payload), &init); err != nil {
			t.Fatalf("pending initializer is not JSON: %v", err)
		}
		if init.SelectionVersion != 0 {
			t.Fatalf("pending rendered version = %d, want 0", init.SelectionVersion)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the pending initializer")
	}

	inCh <- wire.WsMsg{Jid: pendingElem.Jid(), What: what.Input, Data: selectionSyncPrefix + "0"}
	select {
	case msg := <-outCh:
		payload, ok := strings.CutPrefix(msg.Data, "jawstreeSetSelection=")
		if msg.What != what.Call || msg.Jid != 0 || !ok {
			t.Fatalf("selection synchronization = %+v, want request-scoped Call", msg)
		}
		var call jawstreeSelectionCall
		if err = json.Unmarshal([]byte(payload), &call); err != nil {
			t.Fatalf("selection synchronization is not JSON: %v", err)
		}
		if call.Key != tree.key || call.SelectionVersion != 1 || !reflect.DeepEqual(call.Selected, []string{"children.1"}) {
			t.Fatalf("selection synchronization = %+v, want tree %q version 1 selecting b", call, tree.key)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for selection synchronization")
	}
	select {
	case value := <-callerInput:
		t.Fatalf("private selection synchronization reached caller Input handler with %q", value)
	default:
	}

	// Repeating the handshake at the current version is the common fast path and
	// must not walk or resend the selection.
	inCh <- wire.WsMsg{Jid: pendingElem.Jid(), What: what.Input, Data: selectionSyncPrefix + "1"}

	// The private handler must fall through for ordinary events so existing caller
	// handlers retain their normal first-chance semantics. Its completion is also
	// an event-goroutine barrier behind the equal-version synchronization above.
	inCh <- wire.WsMsg{Jid: pendingElem.Jid(), What: what.Set, Data: "children.0.selected=true"}
	select {
	case value := <-callerInput:
		if value != "children.0.selected=true" {
			t.Fatalf("caller Input value = %q", value)
		}
	case <-time.After(time.Second):
		t.Fatal("ordinary Set did not reach the caller Input handler")
	}
	jw.JsCall(pending.JawsKey, "jawstreeSelectionSyncPendingBarrier", "null")
	for {
		select {
		case msg := <-outCh:
			if strings.HasPrefix(msg.Data, "jawstreeSetSelection=") {
				t.Fatalf("equal-version synchronization resent selection: %+v", msg)
			}
			if msg.Data == "jawstreeSelectionSyncPendingBarrier=null" {
				goto pendingBarrierReached
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for the pending-request barrier")
		}
	}

pendingBarrierReached:

	// A request-key barrier deterministically proves the catch-up was not leaked
	// to the already-current peer: any earlier broadcast to that peer is ordered
	// before this Call on its subscription channel.
	jw.JsCall(active.JawsKey, "jawstreeSelectionSyncBarrier", "null")
	for {
		select {
		case msg := <-active.OutCh:
			if strings.HasPrefix(msg.Data, "jawstreeSetSelection=") {
				t.Fatalf("request-scoped selection synchronization leaked to peer: %+v", msg)
			}
			if msg.Data == "jawstreeSelectionSyncBarrier=null" {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for the peer barrier")
		}
	}
}
