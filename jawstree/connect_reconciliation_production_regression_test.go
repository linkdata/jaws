package jawstree

import (
	"net/http"
	"net/http/httptest"
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

func receiveTreeConnectMessage(t *testing.T, ch <-chan wire.WsMsg) (msg wire.WsMsg) {
	t.Helper()
	select {
	case msg = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Tree connection message")
	}
	return
}

func TestTree_ConnectUpdateClosesRenderToSubscribeGap(t *testing.T) {
	for _, tc := range []struct {
		name   string
		option Option
	}{
		{name: "multi-select", option: MultiSelectEnabled},
		{name: "cascade-select", option: CascadeSelectChildren},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)
			go jw.Serve()

			active := jawstest.NewTestRequest(jw, nil)
			defer func() {
				active.Close()
				<-active.DoneCh
			}()
			<-active.ReadyCh

			root := &Node{Children: []*Node{{Name: "a"}, {Name: "b"}}}
			var mu sync.RWMutex
			tree := New(ui.NewJsVar(&mu, root), tc.option)
			activeElem := active.NewElement(tree)
			if err = activeElem.JawsRender(&strings.Builder{}, nil); err != nil {
				t.Fatal(err)
			}
			active.InCh <- wire.WsMsg{}
			if msg := receiveTreeConnectMessage(t, active.OutCh); msg.What != what.Call || msg.Data != "jawstreeInit="+string(tree.appendInitCallData(nil, activeElem.Jid(), 0)) {
				t.Fatalf("active initializer = %#v", msg)
			}

			pending := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
			pendingElem := pending.NewElement(tree)
			if err = pendingElem.JawsRender(&strings.Builder{}, nil); err != nil {
				t.Fatal(err)
			}

			// This is an ordinary supported browser selection. Its JsVar path and
			// Treeview broadcasts correctly reach active subscriptions only,
			// leaving the pending page with its already-rendered selection.
			active.InCh <- wire.WsMsg{Jid: activeElem.Jid(), What: what.Set, Data: "children.1.selected=true"}
			var sawSet bool
			var sawPathSet bool
			for !sawPathSet {
				activeUpdate := receiveTreeConnectMessage(t, active.OutCh)
				switch {
				case activeUpdate.What == what.Set && activeUpdate.Data == "children.1.selected=true":
					sawSet = true
				case activeUpdate.What == what.Call && strings.HasPrefix(activeUpdate.Data, "jawstreeSetPath="):
					if !sawSet {
						t.Fatal("Treeview update arrived before its JsVar path update")
					}
					sawPathSet = true
				default:
					t.Fatalf("active selection update = %#v", activeUpdate)
				}
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

			tree.RLock()
			currentData := tree.Ptr.marshalJSON(nil)
			selectionVersion := tree.selectionVersion
			tree.RUnlock()
			want := []wire.WsMsg{
				{
					Jid:  pendingElem.Jid(),
					What: what.Call,
					Data: "jawstreeInit=" + string(tree.appendInitCallData(nil, pendingElem.Jid(), 0)),
				},
				{
					Jid:  pendingElem.Jid(),
					What: what.Call,
					Data: "jawstreeSet=" + string(tree.appendSetCallData(nil, currentData, selectionVersion)),
				},
			}
			for i := range want {
				if msg := receiveTreeConnectMessage(t, outCh); msg != want[i] {
					t.Fatalf("pending message %d = %#v, want %#v", i, msg, want[i])
				}
			}
			jw.JsCall(pending.JawsKey, "pendingTreeConnectBarrier", "null")
			if msg := receiveTreeConnectMessage(t, outCh); msg.What != what.Call || msg.Jid != 0 || msg.Data != "pendingTreeConnectBarrier=null" {
				t.Fatalf("unexpected pending message after reconciliation: %#v", msg)
			}

			// The catch-up is request-local and must not rebuild an already-current
			// peer Tree.
			jw.JsCall(active.JawsKey, "jawstreeConnectBarrier", "null")
			for {
				msg := receiveTreeConnectMessage(t, active.OutCh)
				if strings.HasPrefix(msg.Data, "jawstreeSet=") {
					t.Fatalf("connection reconciliation leaked to active peer: %#v", msg)
				}
				if msg.What == what.Call && msg.Data == "jawstreeConnectBarrier=null" {
					break
				}
			}
		})
	}
}

func TestTree_ConnectUpdateSkipsUnchangedTree(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	root := &Node{Children: []*Node{{Name: "a"}}}
	var mu sync.RWMutex
	tree := New(ui.NewJsVar(&mu, root), MultiSelectEnabled)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(tree)
	if err = elem.JawsRender(&strings.Builder{}, nil); err != nil {
		t.Fatal(err)
	}

	inCh, outCh, _, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		close(inCh)
		<-doneCh
	}()
	<-readyCh

	if msg := receiveTreeConnectMessage(t, outCh); msg.What != what.Call || msg.Data != "jawstreeInit="+string(tree.appendInitCallData(nil, elem.Jid(), 0)) {
		t.Fatalf("initializer = %#v", msg)
	}
	jw.JsCall(rq.JawsKey, "unchangedTreeConnectBarrier", "null")
	if msg := receiveTreeConnectMessage(t, outCh); msg.What != what.Call || msg.Data != "unchangedTreeConnectBarrier=null" {
		t.Fatalf("unchanged Tree queued a connection update before barrier: %#v", msg)
	}
}
