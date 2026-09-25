package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type orderedMarshalValue struct {
	Text    string
	Entered chan<- struct{}
	Release <-chan struct{}
	Err     error
	Panic   any
}

func (value orderedMarshalValue) MarshalJSON() (data []byte, err error) {
	if value.Entered != nil {
		close(value.Entered)
		<-value.Release
	}
	if value.Err != nil {
		return nil, value.Err
	}
	if value.Panic != nil {
		panic(value.Panic)
	}
	return json.Marshal(value.Text)
}

type orderedPathState struct {
	Value string `json:"value"`
}

func (state *orderedPathState) JawsSetPath(_ *jaws.Element, jsPath string, value any) (err error) {
	if jsPath != "value" {
		return fmt.Errorf("unexpected path %q", jsPath)
	}
	marshaled, ok := value.(orderedMarshalValue)
	if !ok {
		return fmt.Errorf("unexpected value type %T", value)
	}
	if state.Value == marshaled.Text {
		return jaws.ErrValueUnchanged
	}
	state.Value = marshaled.Text
	return
}

func newOrderedJsVar(t *testing.T) (jsvar *JsVar[orderedPathState], elem *jaws.Element, tr *jawstest.TestRequest) {
	t.Helper()

	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr = jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("nil test request")
	}
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	var mu sync.Mutex
	state := orderedPathState{Value: "initial"}
	jsvar = NewJsVar(&mu, &state)
	elem = tr.NewElement(jsvar)
	if err = elem.JawsRender(io.Discard, []any{"ordered"}); err != nil {
		t.Fatal(err)
	}
	return
}

func readJsVarMessage(t *testing.T, tr *jawstest.TestRequest, want what.What) (data string) {
	t.Helper()
	for {
		select {
		case <-t.Context().Done():
			t.Fatalf("timed out waiting for JsVar %s message", want)
		case msg := <-tr.OutCh:
			if msg.What == want {
				return msg.Data
			}
		}
	}
}

func readJsVarSet(t *testing.T, tr *jawstest.TestRequest) (data string) {
	t.Helper()
	return readJsVarMessage(t, tr, what.Set)
}

func TestJsVarConcurrentBroadcastsKeepLatestMutation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jsvar, elem, tr := newOrderedJsVar(t)

		firstEntered := make(chan struct{})
		releaseFirst := make(chan struct{})
		setErr := make(chan error, 2)
		go func() {
			setErr <- jsvar.JawsSetPath(elem, "value", orderedMarshalValue{
				Text:    "first",
				Entered: firstEntered,
				Release: releaseFirst,
			})
		}()
		<-firstEntered

		secondStarted := make(chan struct{})
		go func() {
			close(secondStarted)
			setErr <- jsvar.JawsSetPath(elem, "value", orderedMarshalValue{Text: "second"})
		}()
		<-secondStarted
		close(releaseFirst)

		for range 2 {
			if err := <-setErr; err != nil {
				t.Fatal(err)
			}
		}

		synctest.Wait()
		time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
		synctest.Wait()
		if got := readJsVarSet(t, tr); got != `value="second"` {
			t.Fatalf("broadcast = %q, want latest mutation", got)
		}
		select {
		case msg := <-tr.OutCh:
			t.Fatalf("unexpected extra broadcast: %#v", msg)
		default:
		}
		if current := jsvar.JawsGet(elem).Value; current != "second" {
			t.Fatalf("bound value = %q, want latest mutation %q", current, "second")
		}
	})
}

func TestJsVarMarshalErrorReleasesSetterOrder(t *testing.T) {
	jsvar, elem, tr := newOrderedJsVar(t)

	errMarshal := errors.New("marshal failed")
	err := jsvar.JawsSetPath(elem, "value", orderedMarshalValue{Text: "failed", Err: errMarshal})
	if !errors.Is(err, errMarshal) {
		t.Fatalf("JawsSetPath error = %v, want %v", err, errMarshal)
	}

	setErr := make(chan error, 1)
	go func() {
		setErr <- jsvar.JawsSetPath(elem, "value", orderedMarshalValue{Text: "recovered"})
	}()
	select {
	case <-t.Context().Done():
		t.Fatal("setter remained blocked after JSON marshal failure")
	case err = <-setErr:
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := readJsVarSet(t, tr); got != `value="recovered"` {
		t.Fatalf("broadcast = %q, want recovered value", got)
	}
}

func TestJsVarRecoveredMarshalPanicReleasesSetterOrder(t *testing.T) {
	jsvar, elem, tr := newOrderedJsVar(t)

	errMarshalPanic := errors.New("marshal panic")
	handler := New("set").Clicked(func(Object, *jaws.Element, jaws.Click) (err error) {
		return jsvar.JawsSetPath(elem, "value", orderedMarshalValue{
			Text:  "panicked",
			Panic: errMarshalPanic,
		})
	})
	handlerElem := tr.NewElement(NewButton(handler))
	if err := handlerElem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}

	tr.InCh <- wire.WsMsg{
		Jid:  handlerElem.Jid(),
		What: what.Click,
		Data: "0 0 0 set",
	}
	if alert := readJsVarMessage(t, tr, what.Alert); alert != "danger\nevent handler failed" {
		t.Fatalf("recovered panic alert = %q, want generic danger alert", alert)
	}

	setErr := make(chan error, 1)
	go func() {
		setErr <- jsvar.JawsSetPath(elem, "value", orderedMarshalValue{Text: "recovered"})
	}()
	select {
	case <-t.Context().Done():
		t.Fatal("setter remained blocked after recovered JSON marshal panic")
	case err := <-setErr:
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := readJsVarSet(t, tr); got != `value="recovered"` {
		t.Fatalf("broadcast = %q, want recovered value", got)
	}
}

type reentrantPathState struct {
	Value       string `json:"value"`
	jsvar       *JsVar[reentrantPathState]
	callbacks   int
	callbackErr error
}

func (state *reentrantPathState) JawsSetPath(_ *jaws.Element, jsPath string, value any) (err error) {
	if jsPath != "value" {
		return fmt.Errorf("unexpected path %q", jsPath)
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("unexpected value type %T", value)
	}
	if state.Value == text {
		return jaws.ErrValueUnchanged
	}
	state.Value = text
	return
}

func (state *reentrantPathState) JawsPathSet(elem *jaws.Element, jsPath string, _ any) {
	state.callbacks++
	if state.callbacks == 1 {
		state.callbackErr = state.jsvar.JawsSetPath(elem, jsPath, "callback")
	}
}

func TestJsVarSetPatherCanReenterSetter(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("nil test request")
	}
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
	<-tr.ReadyCh

	var mu sync.Mutex
	state := reentrantPathState{Value: "initial"}
	jsvar := NewJsVar(&mu, &state)
	state.jsvar = jsvar
	elem := tr.NewElement(jsvar)
	if err = elem.JawsRender(io.Discard, []any{"reentrant"}); err != nil {
		t.Fatal(err)
	}

	setErr := make(chan error, 1)
	go func() {
		setErr <- jsvar.JawsSetPath(elem, "value", "outer")
	}()
	select {
	case <-t.Context().Done():
		t.Fatal("SetPather callback deadlocked while re-entering the JsVar setter")
	case err = <-setErr:
		if err != nil {
			t.Fatal(err)
		}
	}
	if state.callbackErr != nil {
		t.Fatalf("reentrant setter failed: %v", state.callbackErr)
	}
	if state.callbacks != 2 || state.Value != "callback" {
		t.Fatalf("state after reentrant callback = %#v", state)
	}

	if got := readJsVarSet(t, tr); got != `value="callback"` {
		t.Fatalf("broadcast = %q, want callback mutation", got)
	}
}
