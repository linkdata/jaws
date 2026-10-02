package ui

import (
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type multiSelectTestHandler struct {
	children      []jaws.UI
	values        []string
	containsCalls int
	getCalls      int
}

func (h *multiSelectTestHandler) JawsContains(*jaws.Element) []jaws.UI {
	h.containsCalls++
	return h.children
}

func (h *multiSelectTestHandler) JawsGetValues(*jaws.Element) []string {
	h.getCalls++
	return h.values
}

func (*multiSelectTestHandler) JawsSetValues(*jaws.Element, []string) error {
	return jaws.ErrValueUnchanged
}

type multiSelectTestSource struct {
	*named.BoolArray
	setCalls atomic.Int32
	setError error
}

func (s *multiSelectTestSource) JawsSetValues(elem *jaws.Element, values []string) error {
	s.setCalls.Add(1)
	if s.setError != nil {
		return s.setError
	}
	return s.BoolArray.JawsSetValues(elem, values)
}

func TestMultiSelectInitialValues(t *testing.T) {
	checked := named.NewBoolArray(true).Add("1", "one").Add("2", "two")
	checked.Set("1", true)
	checked.Set("2", true)
	for _, tt := range []struct {
		name     string
		handler  named.MultiSelectHandler
		want     string
		selected int
	}{
		{name: "checked", handler: checked, want: `["1","2"]`, selected: 2},
		{name: "empty", handler: named.NewBoolArray(true).Add("1", "one"), want: `[]`},
		{name: "custom getter", handler: &multiSelectTestHandler{
			children: []jaws.UI{plainSelectOption{value: "1", label: "one"}, plainSelectOption{value: "2", label: "two"}},
			values:   []string{"2"},
		}, want: `["2"]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tr := newNumberRangeLiveRequest(t, nil)
			rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
			if err := rw.MultiSelect(tt.handler, `class="choices"`); err != nil {
				t.Fatal(err)
			}
			markup := tr.BodyString()
			if !strings.HasPrefix(markup, "<select ") || !strings.Contains(markup, " multiple") || !strings.Contains(markup, `class="choices"`) {
				t.Fatalf("MultiSelect markup = %q", markup)
			}
			if got := strings.Count(markup, " selected"); got != tt.selected {
				t.Fatalf("selected options = %d, want %d", got, tt.selected)
			}
			elems := tr.GetElements(tt.handler)
			if len(elems) != 1 {
				t.Fatalf("handler elements = %d, want 1", len(elems))
			}
			tr.InCh <- wire.WsMsg{}
			awaitNumberRangeValue(t, tr, elems[0], tt.want)
		})
	}
}

func TestMultiSelectUpdatesOptionsBeforeValues(t *testing.T) {
	tr := newNumberRangeLiveRequest(t, nil)
	one := plainSelectOption{value: "1", label: "one"}
	two := plainSelectOption{value: "2", label: "two"}
	three := plainSelectOption{value: "3", label: "three"}
	handler := &multiSelectTestHandler{children: []jaws.UI{one, two}, values: []string{"1", "2"}}
	widget := NewMultiSelect(handler)
	elem, _ := renderUI(t, tr.Request, widget)
	before := containerElements(t, elem)
	tr.InCh <- wire.WsMsg{}
	awaitNumberRangeValue(t, tr, elem, `["1","2"]`)

	handler.children = []jaws.UI{two, three, one}
	handler.values = []string{"2", "3"}
	widget.JawsUpdate(elem)
	after := containerElements(t, elem)
	if len(after) != 3 || after[0] != before[1] || after[2] != before[0] {
		t.Fatal("reordering did not retain existing option Elements")
	}
	tr.InCh <- wire.WsMsg{}
	sawAppend, sawOrder := false, false
	for {
		select {
		case msg := <-tr.OutCh:
			switch msg.What {
			case what.Append:
				sawAppend = true
				if !strings.Contains(msg.Data, `value="3"`) {
					t.Fatalf("Append lacks new option: %q", msg.Data)
				}
			case what.Order:
				sawOrder = true
			case what.Value:
				if msg.Jid != elem.Jid() || msg.Data != `["2","3"]` || !sawAppend || !sawOrder {
					t.Fatalf("Value = %+v, append=%v order=%v; want complete selection after options", msg, sawAppend, sawOrder)
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("no MultiSelect value update")
		}
	}
}

func TestMultiSelectInputReconciliation(t *testing.T) {
	setError := errors.New("selection rejected")
	for _, tt := range []struct {
		name       string
		input      string
		want       string
		setCalls   int32
		setError   error
		updatePeer bool
	}{
		{name: "accepted canonical values", input: `["3","2","missing","2"]`, want: `["2","3"]`, setCalls: 1, updatePeer: true},
		{name: "unchanged", input: `["2","1","1"]`, want: `["1","2"]`, setCalls: 1},
		{name: "empty selection", input: `[]`, want: `[]`, setCalls: 1, updatePeer: true},
		{name: "setter rejection", input: `["3"]`, want: `["1","2"]`, setCalls: 1, setError: setError, updatePeer: true},
		{name: "malformed", input: `[`, want: `["1","2"]`},
		{name: "empty payload", want: `["1","2"]`},
		{name: "string", input: `"1"`, want: `["1","2"]`},
		{name: "object", input: `{}`, want: `["1","2"]`},
		{name: "null", input: `null`, want: `["1","2"]`},
		{name: "null item", input: `[null]`, want: `["1","2"]`},
		{name: "non-string item", input: `[1]`, want: `["1","2"]`},
		{name: "empty item", input: `["1",""]`, want: `["1","2"]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tr := newNumberRangeLiveRequest(t, new(numberRangeLogger))
				source := &multiSelectTestSource{
					BoolArray: named.NewBoolArray(true).Add("1", "one").Add("2", "two").Add("3", "three"),
					setError:  tt.setError,
				}
				source.Set("1", true)
				source.Set("2", true)
				origin, _ := renderUI(t, tr.Request, NewMultiSelect(source))
				peer, _ := renderUI(t, tr.Request, NewMultiSelect(source))
				if origin.UI() != peer.UI() || requireContainerState(t, origin) == requireContainerState(t, peer) {
					t.Fatal("equal MultiSelect definitions must keep separate Element states")
				}
				tr.InCh <- wire.WsMsg{}
				awaitNumberRangeValue(t, tr, origin, `["1","2"]`)
				awaitNumberRangeValue(t, tr, peer, `["1","2"]`)

				tr.InCh <- wire.WsMsg{Jid: origin.Jid(), What: what.Input, Data: tt.input}
				synctest.Wait()
				time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
				synctest.Wait()
				originUpdates, peerUpdates, alerts := 0, 0, 0
				for len(tr.OutCh) > 0 {
					msg := <-tr.OutCh
					if msg.What == what.Alert {
						alerts++
					}
					if msg.What != what.Value || (msg.Jid != origin.Jid() && msg.Jid != peer.Jid()) {
						continue
					}
					if msg.Data != tt.want {
						t.Fatalf("reconciled selection = %q, want %q", msg.Data, tt.want)
					}
					if msg.Jid == origin.Jid() {
						originUpdates++
					} else {
						peerUpdates++
					}
				}
				if originUpdates == 0 || (peerUpdates > 0) != tt.updatePeer {
					t.Fatalf("origin updates = %d, peer updates = %d; want source correction, peer=%v", originUpdates, peerUpdates, tt.updatePeer)
				}
				if (alerts > 0) != (tt.setError != nil) {
					t.Fatalf("alerts = %d, setter error = %v", alerts, tt.setError)
				}
				if got := source.setCalls.Load(); got != tt.setCalls {
					t.Fatalf("setter calls = %d, want %d", got, tt.setCalls)
				}
			})
		})
	}
}

func TestMultiSelectRenderFailureDoesNotReadValues(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		t.Run(map[bool]string{false: "child error", true: "occupied state"}[occupied], func(t *testing.T) {
			tr := newNumberRangeLiveRequest(t, nil)
			wantErr := errors.New("option render failed")
			handler := &multiSelectTestHandler{children: []jaws.UI{testRenderErrorUI{err: wantErr}}}
			widget := NewMultiSelect(handler)
			elem := tr.NewElement(widget)
			if occupied {
				wantErr = jaws.ErrElementStateClaimed
				if err := jaws.SetElementState(elem, new(int)); err != nil {
					t.Fatal(err)
				}
			}
			if err := widget.JawsRender(elem, io.Discard, nil); !errors.Is(err, wantErr) {
				t.Fatalf("render error = %v, want %v", err, wantErr)
			}
			if handler.getCalls != 0 || (occupied && handler.containsCalls != 0) {
				t.Fatalf("callbacks after failed render: getter=%d children=%d", handler.getCalls, handler.containsCalls)
			}
		})
	}
}
