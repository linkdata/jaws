package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jq"
)

type jsVarCheckedState struct {
	Value        string `json:"value"`
	pathSetCalls int
}

func (state *jsVarCheckedState) JawsPathSet(*jaws.Element, string, any) {
	state.pathSetCalls++
}

type jsVarSliceData struct {
	Items []string `json:"items"`
}

type jsVarConcurrentData struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

var errJsVarMarshalFailure = fmt.Errorf("jsvar marshal failure: %w", errors.Join(jaws.ErrValueUnchanged, jaws.ErrEventUnhandled))

type jsVarMarshalFailureData struct {
	Items []string `json:"items"`
}

func (data jsVarMarshalFailureData) MarshalJSON() ([]byte, error) {
	if len(data.Items) > 0 {
		return nil, errJsVarMarshalFailure
	}
	type plain jsVarMarshalFailureData
	return json.Marshal(plain(data))
}

type jsVarAppendPathSetter struct {
	Items []string `json:"items"`
}

func (data *jsVarAppendPathSetter) JawsSetPath(_ *jaws.Element, _ string, value any) error {
	data.Items = append(data.Items, fmt.Sprint(value))
	return nil
}

var errJsVarPathSetterTooLarge = fmt.Errorf("PathSetter rejected: %w", ErrJsVarTooLarge)

type jsVarSizeErrorPathSetter struct{}

func (*jsVarSizeErrorPathSetter) JawsSetPath(*jaws.Element, string, any) error {
	return errJsVarPathSetterTooLarge
}

type jsVarErrorPathSetter struct {
	err error
}

func (data *jsVarErrorPathSetter) JawsSetPath(*jaws.Element, string, any) error {
	return data.err
}

type jsVarIsLockProbeError struct {
	mu             *sync.Mutex
	setMu          *sync.Mutex
	match          error
	locksAvailable bool
}

func (*jsVarIsLockProbeError) Error() string {
	return "lock probe"
}

func (err *jsVarIsLockProbeError) Is(target error) bool {
	setMuOK := err.setMu.TryLock()
	if setMuOK {
		err.setMu.Unlock()
	}
	muOK := err.mu.TryLock()
	if muOK {
		err.mu.Unlock()
	}
	err.locksAvailable = setMuOK && muOK
	return target == err.match
}

func clientSetFrame(t *testing.T, jsvar IsJsVar, elem *jaws.Element, jsPath string, value any) error {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return jaws.CallEventHandlers(jsvar, elem, what.Set, jsPath+"="+string(data))
}

func cleanupJsVarTestRequest(t *testing.T, tr *jawstest.TestRequest) {
	t.Helper()
	t.Cleanup(func() {
		tr.Close()
		<-tr.DoneCh
	})
}

func TestJSONSizeCheck(t *testing.T) {
	value := "x"
	data, err := json.Marshal(&value)
	if err != nil {
		t.Fatal(err)
	}

	if check := JSONSizeCheck[string](0); check != nil {
		t.Fatal("zero maximum returned a non-nil check")
	}
	if check := JSONSizeCheck[string](-1); check != nil {
		t.Fatal("negative maximum returned a non-nil check")
	}
	if err = JSONSizeCheck[string](len(data))(&value, "value"); err != nil {
		t.Fatalf("exact boundary rejected: %v", err)
	}
	if err = JSONSizeCheck[string](len(data)-1)(&value, "value"); !errors.Is(err, ErrJsVarTooLarge) {
		t.Fatalf("over-limit check error = %v, want ErrJsVarTooLarge", err)
	}
	if err = JSONSizeCheck[string](math.MaxInt)(&value, "value"); err != nil {
		t.Fatalf("MaxInt check rejected: %v", err)
	}

	bad := jsVarMarshalFailureData{Items: []string{"x"}}
	err = JSONSizeCheck[jsVarMarshalFailureData](1024)(&bad, "items.0")
	if !errors.Is(err, ErrJsVarTooLarge) || !errors.Is(err, errJsVarMarshalFailure) {
		t.Fatalf("marshal check error = %v, want ErrJsVarTooLarge and marshal failure", err)
	}
}

func TestJsVarClientCheckContract(t *testing.T) {
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
	cleanupJsVarTestRequest(t, tr)
	<-tr.ReadyCh

	var mu sync.Mutex
	state := jsVarCheckedState{Value: "initial"}
	jsvar := NewJsVar(&mu, &state)
	errRejected := fmt.Errorf("blocked: %w", jaws.ErrValueUnchanged)
	checkCalls := 0
	checkHeldLock := true
	jsvar.ClientCheck = func(value *jsVarCheckedState, jsPath string) error {
		checkCalls++
		if value != &state {
			t.Errorf("check value = %p, want %p", value, &state)
		}
		if mu.TryLock() {
			checkHeldLock = false
			mu.Unlock()
		}
		if jsPath != "value" {
			t.Errorf("check path = %q, want value", jsPath)
		}
		if value.Value == "blocked" {
			return errRejected
		}
		return nil
	}

	rw := RequestWriter{Request: tr.Request, Writer: io.Discard}
	if err = rw.JsVar("checked", jsvar); err != nil {
		t.Fatal(err)
	}
	elements := tr.GetElements(&state)
	if len(elements) != 1 {
		t.Fatalf("rendered elements = %d, want 1", len(elements))
	}
	elem := elements[0]

	if err = clientSetFrame(t, jsvar, elem, "value", "blocked"); err != errRejected {
		t.Fatalf("rejection error = %v, want exact %v", err, errRejected)
	}
	if state.Value != "initial" || state.pathSetCalls != 0 {
		t.Fatalf("rejected state = %#v, want initial value and no callback", state)
	}
	if !checkHeldLock {
		t.Fatal("ClientCheck ran without the backing write lock")
	}
	if tr.Context().Err() != nil {
		t.Fatalf("ordinary rejection cancelled request: %v", tr.Context().Err())
	}
	select {
	case msg := <-tr.OutCh:
		t.Fatalf("rejected write broadcast %#v", msg)
	default:
	}

	if err = clientSetFrame(t, jsvar, elem, "value", "accepted"); err != nil {
		t.Fatal(err)
	}
	if state.Value != "accepted" || state.pathSetCalls != 1 {
		t.Fatalf("accepted state = %#v, want accepted value and one callback", state)
	}
	msg := awaitJsVarOperation(t, "accepted ClientCheck broadcast", tr.OutCh)
	if msg.What != what.Set || msg.Data != `value="accepted"` {
		t.Fatalf("broadcast = %#v, want value=accepted Set", msg)
	}

	if err = clientSetFrame(t, jsvar, elem, "value", "accepted"); err != nil {
		t.Fatalf("unchanged browser write error = %v", err)
	}
	if checkCalls != 2 || state.pathSetCalls != 1 {
		t.Fatalf("after no-op: check calls = %d, path callbacks = %d; want 2, 1", checkCalls, state.pathSetCalls)
	}
	select {
	case msg := <-tr.OutCh:
		t.Fatalf("unchanged write broadcast %#v", msg)
	default:
	}

	if err = clientSetFrame(t, jsvar, elem, "value", 1); !errors.Is(err, jq.ErrTypeMismatch) {
		t.Fatalf("invalid write error = %v, want ErrTypeMismatch", err)
	}
	if checkCalls != 2 || state.Value != "accepted" || state.pathSetCalls != 1 {
		t.Fatalf("invalid write changed state/check counts: %#v, %d", state, checkCalls)
	}

	if err = clientSetFrame(t, jsvar, elem, "value", "barrier"); err != nil {
		t.Fatal(err)
	}
	msg = awaitJsVarOperation(t, "broadcast barrier after no-op and invalid writes", tr.OutCh)
	if msg.What != what.Set || msg.Data != `value="barrier"` {
		t.Fatalf("first broadcast after no-op and invalid writes = %#v, want value=barrier Set", msg)
	}
	if checkCalls != 3 || state.pathSetCalls != 2 {
		t.Fatalf("after barrier: check calls = %d, path callbacks = %d; want 3, 2", checkCalls, state.pathSetCalls)
	}
}

func TestJsVarClientCheckRejectRestoresZeroValueAppendAliases(t *testing.T) {
	backing := []string{"sentinel"}
	state := jsVarSliceData{Items: backing[:0]}
	var mu sync.Mutex
	jsvar := NewJsVar(&mu, &state)
	errRejected := errors.New("rejected")
	checkCalls := 0
	jsvar.ClientCheck = func(value *jsVarSliceData, jsPath string) error {
		checkCalls++
		if jsPath != "items.0" {
			t.Errorf("check path = %q, want items.0", jsPath)
		}
		if len(value.Items) != 1 || value.Items[0] != "" {
			t.Errorf("tentative items = %#v, want one empty item", value.Items)
		}
		return errRejected
	}

	if err := jsvar.JawsInput(nil, `items.0=""`); err != errRejected {
		t.Fatalf("rejection error = %v, want exact %v", err, errRejected)
	}
	if checkCalls != 1 {
		t.Fatalf("check calls = %d, want 1", checkCalls)
	}
	if len(state.Items) != 0 || backing[0] != "sentinel" {
		t.Fatalf("rollback left items=%#v backing=%#v", state.Items, backing)
	}
}

func TestJsVarClientCheckRootPath(t *testing.T) {
	for _, jsPath := range []string{"", "."} {
		t.Run(fmt.Sprintf("path_%q", jsPath), func(t *testing.T) {
			state := "old"
			jsvar := NewJsVar(new(sync.Mutex), &state)
			gotPath := "not called"
			jsvar.ClientCheck = func(_ *string, jsPath string) error {
				gotPath = jsPath
				return nil
			}
			if err := jsvar.JawsInput(nil, jsPath+`="new"`); err != nil {
				t.Fatal(err)
			}
			if gotPath != jsPath || state != "new" {
				t.Fatalf("root check path, state = %q, %q; want %q, new", gotPath, state, jsPath)
			}
		})
	}
}

func TestJsVarClientCheckReceivesNoncanonicalPathUnchanged(t *testing.T) {
	state := jsVarCheckedState{Value: "old"}
	jsvar := NewJsVar(new(sync.Mutex), &state)
	gotPath := "not called"
	jsvar.ClientCheck = func(_ *jsVarCheckedState, jsPath string) error {
		gotPath = jsPath
		return nil
	}
	const jsPath = ".value."
	if err := jsvar.JawsInput(nil, jsPath+`="new"`); err != nil {
		t.Fatal(err)
	}
	if gotPath != jsPath || state.Value != "new" {
		t.Fatalf("check path, value = %q, %q; want %q, new", gotPath, state.Value, jsPath)
	}
}

func TestJsVarClientCheckPanicRollsBackAndReleasesLocks(t *testing.T) {
	_, rq := newCoreRequest(t)
	backing := []string{"sentinel"}
	state := jsVarSliceData{Items: backing[:0]}
	var mu sync.Mutex
	jsvar := NewJsVar(&mu, &state)
	panicErr := errors.New("check panic")
	checkCalls := 0
	jsvar.ClientCheck = func(*jsVarSliceData, string) error {
		checkCalls++
		if checkCalls == 1 {
			panic(panicErr)
		}
		return nil
	}
	elem := rq.NewElement(jsvar)

	err := jaws.CallEventHandlers(jsvar, elem, what.Set, `items.0=""`)
	if !errors.Is(err, jaws.ErrEventHandlerPanic) || !errors.Is(err, panicErr) {
		t.Fatalf("panic error = %v, want ErrEventHandlerPanic and panic value", err)
	}
	if len(state.Items) != 0 || backing[0] != "sentinel" {
		t.Fatalf("panic rollback left items=%#v backing=%#v", state.Items, backing)
	}
	if rq.Context().Err() != nil {
		t.Fatalf("check panic cancelled request: %v", rq.Context().Err())
	}

	done := make(chan error, 1)
	go func() {
		done <- jsvar.JawsInput(elem, `items.0="accepted"`)
	}()
	if err = awaitJsVarOperation(t, "write after ClientCheck panic", done); err != nil {
		t.Fatal(err)
	}
	if len(state.Items) != 1 || state.Items[0] != "accepted" {
		t.Fatalf("write after panic left items=%#v", state.Items)
	}
	if checkCalls != 2 {
		t.Fatalf("check calls after recovery write = %d, want 2", checkCalls)
	}
}

type jsVarLockProbeLogger struct {
	jsvar  *JsVar[jsVarSliceData]
	mu     *sync.Mutex
	result chan [2]bool
}

func (*jsVarLockProbeLogger) Info(string, ...any) {}
func (*jsVarLockProbeLogger) Warn(string, ...any) {}

func (logger *jsVarLockProbeLogger) Error(string, ...any) {
	setMuOK := logger.jsvar.setMu.TryLock()
	if setMuOK {
		logger.jsvar.setMu.Unlock()
	}
	muOK := logger.mu.TryLock()
	if muOK {
		logger.mu.Unlock()
	}
	select {
	case logger.result <- [2]bool{setMuOK, muOK}:
	default:
	}
}

func TestJsVarJSONSizeCheckRejectsAndCancelsOutsideLocks(t *testing.T) {
	jw, rq := newCoreRequest(t)
	go jw.Serve()

	var mu sync.Mutex
	state := jsVarSliceData{}
	candidate := jsVarSliceData{Items: []string{"0123456789"}}
	data, err := json.Marshal(&candidate)
	if err != nil {
		t.Fatal(err)
	}
	jsvar := NewJsVar(&mu, &state)
	jsvar.ClientCheck = JSONSizeCheck[jsVarSliceData](len(data) - 1)
	result := make(chan [2]bool, 1)
	jw.Logger = &jsVarLockProbeLogger{jsvar: jsvar, mu: &mu, result: result}
	elem := rq.NewElement(jsvar)

	if err = jsvar.JawsInput(elem, `items.0="0123456789"`); err != ErrJsVarTooLarge {
		t.Fatalf("client write error = %v, want exact ErrJsVarTooLarge", err)
	}
	if len(state.Items) != 0 {
		t.Fatalf("rejected size check left items=%#v", state.Items)
	}
	if cause := context.Cause(rq.Context()); !errors.Is(cause, ErrJsVarTooLarge) || !strings.Contains(cause.Error(), "serialized size") {
		t.Fatalf("cancellation cause = %v, want detailed ErrJsVarTooLarge", cause)
	}
	select {
	case locks := <-result:
		if !locks[0] || !locks[1] {
			t.Fatalf("cancellation ran with locks held: setMu=%t backing=%t", !locks[0], !locks[1])
		}
	default:
		t.Fatal("request cancellation was not logged")
	}
}

func TestJsVarJSONSizeCheckMarshalFailureRollsBackAndCancels(t *testing.T) {
	_, rq := newCoreRequest(t)
	var mu sync.Mutex
	state := jsVarMarshalFailureData{}
	jsvar := NewJsVar(&mu, &state)
	jsvar.ClientCheck = JSONSizeCheck[jsVarMarshalFailureData](1024)
	elem := rq.NewElement(jsvar)

	err := jsvar.JawsInput(elem, `items.0="x"`)
	if err != ErrJsVarTooLarge {
		t.Fatalf("client write error = %v, want exact ErrJsVarTooLarge", err)
	}
	if len(state.Items) != 0 {
		t.Fatalf("marshal rejection left items=%#v", state.Items)
	}
	cause := context.Cause(rq.Context())
	if !errors.Is(cause, ErrJsVarTooLarge) || !errors.Is(cause, errJsVarMarshalFailure) {
		t.Fatalf("cancellation cause = %v, want size and marshal errors", cause)
	}
	if errors.Is(err, jaws.ErrValueUnchanged) || errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("public error exposed handler-control identity: %v", err)
	}
}

func TestJsVarJSONSizeCheckSharedBackingState(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr1 := jawstest.NewTestRequest(jw, nil)
	tr2 := jawstest.NewTestRequest(jw, nil)
	if tr1 == nil || tr2 == nil {
		t.Fatal("nil test request")
	}
	cleanupJsVarTestRequest(t, tr1)
	cleanupJsVarTestRequest(t, tr2)
	<-tr1.ReadyCh
	<-tr2.ReadyCh

	var mu sync.Mutex
	state := jsVarSliceData{}
	jsvar1 := NewJsVar(&mu, &state)
	jsvar2 := NewJsVar(&mu, &state)
	jsvar1.ClientCheck = JSONSizeCheck[jsVarSliceData](50)
	jsvar2.ClientCheck = JSONSizeCheck[jsVarSliceData](50)
	rw1 := RequestWriter{Request: tr1.Request, Writer: io.Discard}
	rw2 := RequestWriter{Request: tr2.Request, Writer: io.Discard}
	if err = rw1.JsVar("v1", jsvar1); err != nil {
		t.Fatal(err)
	}
	if err = rw2.JsVar("v2", jsvar2); err != nil {
		t.Fatal(err)
	}
	elements1 := tr1.GetElements(&state)
	elements2 := tr2.GetElements(&state)
	if len(elements1) != 1 || len(elements2) != 1 {
		t.Fatalf("rendered elements = %d, %d; want 1, 1", len(elements1), len(elements2))
	}
	elem1 := elements1[0]
	elem2 := elements2[0]

	const value = "xxxxxxxxxxxxxxxxxxxx"
	if err = clientSetFrame(t, jsvar1, elem1, "items.0", value); err != nil {
		t.Fatal(err)
	}
	if err = clientSetFrame(t, jsvar2, elem2, "items.1", value); err != ErrJsVarTooLarge {
		t.Fatalf("second binding error = %v, want exact ErrJsVarTooLarge", err)
	}
	if len(state.Items) != 1 || state.Items[0] != value {
		t.Fatalf("shared state after rejection = %#v", state)
	}
	data, err := json.Marshal(&state)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 50 {
		t.Fatalf("serialized shared state = %d bytes, want <= 50", len(data))
	}
	if tr1.Context().Err() != nil {
		t.Fatalf("accepted binding request cancelled: %v", tr1.Context().Err())
	}
	if !errors.Is(context.Cause(tr2.Context()), ErrJsVarTooLarge) {
		t.Fatalf("rejected binding cause = %v, want ErrJsVarTooLarge", context.Cause(tr2.Context()))
	}
}

func TestJsVarJSONSizeCheckConcurrentSharedBackingState(t *testing.T) {
	const value = "xxxxxxxxxxxxxxxxxxxx"
	leftOnly := jsVarConcurrentData{Left: value}
	both := jsVarConcurrentData{Left: value, Right: value}
	leftData, err := json.Marshal(&leftOnly)
	if err != nil {
		t.Fatal(err)
	}
	bothData, err := json.Marshal(&both)
	if err != nil {
		t.Fatal(err)
	}
	if len(bothData) <= len(leftData) {
		t.Fatalf("test values do not distinguish one update (%d) from two (%d)", len(leftData), len(bothData))
	}

	var mu sync.Mutex
	state := jsVarConcurrentData{}
	jsvar1 := NewJsVar(&mu, &state)
	jsvar2 := NewJsVar(&mu, &state)
	jsvar1.ClientCheck = JSONSizeCheck[jsVarConcurrentData](len(leftData))
	jsvar2.ClientCheck = JSONSizeCheck[jsVarConcurrentData](len(leftData))

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		results <- jsvar1.JawsInput(nil, `left="xxxxxxxxxxxxxxxxxxxx"`)
	}()
	go func() {
		<-start
		results <- jsvar2.JawsInput(nil, `right="xxxxxxxxxxxxxxxxxxxx"`)
	}()
	close(start)

	succeeded := 0
	rejected := 0
	for range 2 {
		err = awaitJsVarOperation(t, "concurrent shared JsVar write", results)
		switch err {
		case nil:
			succeeded++
		case ErrJsVarTooLarge:
			rejected++
		default:
			t.Fatalf("concurrent write error = %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent writes: succeeded=%d rejected=%d; want 1, 1", succeeded, rejected)
	}
	if (state.Left == value) == (state.Right == value) {
		t.Fatalf("concurrent shared state = %#v, want exactly one field set", state)
	}
	data, err := json.Marshal(&state)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > len(leftData) {
		t.Fatalf("serialized shared state = %d bytes, want <= %d", len(data), len(leftData))
	}
}

func TestJsVarClientCheckBypasses(t *testing.T) {
	t.Run("render", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		var mu sync.Mutex
		state := jsVarSliceData{Items: []string{strings.Repeat("x", 100)}}
		jsvar := NewJsVar(&mu, &state)
		calls := 0
		jsvar.ClientCheck = func(*jsVarSliceData, string) error {
			calls++
			return errors.New("reject")
		}
		if err := jsvar.JawsRender(rq.NewElement(jsvar), io.Discard, []any{"v"}); err != nil {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatalf("render called ClientCheck %d times", calls)
		}
	})

	t.Run("server write", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		var mu sync.Mutex
		state := jsVarCheckedState{Value: "old"}
		jsvar := NewJsVar(&mu, &state)
		calls := 0
		jsvar.ClientCheck = func(*jsVarCheckedState, string) error {
			calls++
			return ErrJsVarTooLarge
		}
		if err := jsvar.JawsSetPath(rq.NewElement(jsvar), "value", "new"); err != nil {
			t.Fatal(err)
		}
		if calls != 0 || state.Value != "new" {
			t.Fatalf("server write state=%#v calls=%d", state, calls)
		}
		if rq.Context().Err() != nil {
			t.Fatalf("server write cancelled request: %v", rq.Context().Err())
		}
	})

	t.Run("PathSetter", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		var mu sync.Mutex
		state := jsVarAppendPathSetter{}
		jsvar := NewJsVar(&mu, &state)
		calls := 0
		jsvar.ClientCheck = func(*jsVarAppendPathSetter, string) error {
			calls++
			return ErrJsVarTooLarge
		}
		if err := jsvar.JawsInput(rq.NewElement(jsvar), `items.0="x"`); err != nil {
			t.Fatal(err)
		}
		if calls != 0 || len(state.Items) != 1 || state.Items[0] != "x" {
			t.Fatalf("PathSetter state=%#v calls=%d", state, calls)
		}
		if rq.Context().Err() != nil {
			t.Fatalf("PathSetter write cancelled request: %v", rq.Context().Err())
		}
	})

	t.Run("unchanged PathSetter", func(t *testing.T) {
		var mu sync.Mutex
		state := jsVarPathHooks{Value: "same"}
		jsvar := NewJsVar(&mu, &state)
		calls := 0
		jsvar.ClientCheck = func(*jsVarPathHooks, string) error {
			calls++
			return errors.New("reject")
		}
		if err := jsvar.JawsInput(nil, `value="same"`); err != nil {
			t.Fatalf("unchanged PathSetter error = %v", err)
		}
		if calls != 0 || state.setCalls != 0 {
			t.Fatalf("unchanged PathSetter state=%#v calls=%d", state, calls)
		}
	})

	t.Run("PathSetter size error", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		state := jsVarSizeErrorPathSetter{}
		jsvar := NewJsVar(new(sync.Mutex), &state)
		err := jsvar.JawsInput(rq.NewElement(jsvar), `value=1`)
		if err != errJsVarPathSetterTooLarge {
			t.Fatalf("PathSetter error = %v, want exact %v", err, errJsVarPathSetterTooLarge)
		}
		if rq.Context().Err() != nil {
			t.Fatalf("PathSetter error cancelled request: %v", rq.Context().Err())
		}
	})
}

func TestJsVarPathSetterUnchangedErrorInspectedOutsideLocks(t *testing.T) {
	var mu sync.Mutex
	state := jsVarErrorPathSetter{}
	jsvar := NewJsVar(&mu, &state)
	probe := &jsVarIsLockProbeError{mu: &mu, setMu: &jsvar.setMu, match: jaws.ErrValueUnchanged}
	state.err = probe

	if err := jsvar.JawsInput(nil, `value=1`); err != nil {
		t.Fatalf("unchanged PathSetter error = %v", err)
	}
	if !probe.locksAvailable {
		t.Fatal("PathSetter error was inspected while a JsVar lock was held")
	}
}

func TestJsVarClientCheckSizeErrorInspectedOutsideLocks(t *testing.T) {
	var mu sync.Mutex
	state := jsVarCheckedState{Value: "old"}
	jsvar := NewJsVar(&mu, &state)
	probe := &jsVarIsLockProbeError{mu: &mu, setMu: &jsvar.setMu, match: ErrJsVarTooLarge}
	jsvar.ClientCheck = func(*jsVarCheckedState, string) error {
		return probe
	}

	if err := jsvar.JawsInput(nil, `value="new"`); err != ErrJsVarTooLarge {
		t.Fatalf("size rejection error = %v, want exact ErrJsVarTooLarge", err)
	}
	if !probe.locksAvailable {
		t.Fatal("ClientCheck size error was inspected while a JsVar lock was held")
	}
	if state.Value != "old" {
		t.Fatalf("size rejection left value %q, want old", state.Value)
	}
}

type jsVarNilPathSetter struct{}

func (jsVarNilPathSetter) JawsSetPath(*jaws.Element, string, any) error {
	panic("called PathSetter through a nil Ptr")
}

func TestJsVarClientCheckNilPtr(t *testing.T) {
	var ptr *jsVarNilPathSetter
	jsvar := NewJsVar(new(sync.Mutex), ptr)
	calls := 0
	jsvar.ClientCheck = func(*jsVarNilPathSetter, string) error {
		calls++
		return nil
	}
	if err := jsvar.JawsInput(nil, `value=1`); !errors.Is(err, jq.ErrInvalidReceiver) {
		t.Fatalf("nil Ptr error = %v, want ErrInvalidReceiver", err)
	}
	if calls != 0 {
		t.Fatalf("nil Ptr called ClientCheck %d times", calls)
	}
}

type jsVarBlockingJSONValue struct {
	entered chan struct{}
	release chan struct{}
}

func (value *jsVarBlockingJSONValue) MarshalJSON() ([]byte, error) {
	close(value.entered)
	<-value.release
	return []byte(`{"ok":true}`), nil
}

type jsVarAliasedValue struct {
	Value any `json:"value"`
}

type jsVarMarshalOutcomeValue struct {
	err        error
	panicValue any
}

func (value *jsVarMarshalOutcomeValue) MarshalJSON() ([]byte, error) {
	if value.panicValue != nil {
		panic(value.panicValue)
	}
	return nil, value.err
}

type jsVarBroadcastPathSetter struct {
	Value any `json:"value"`
}

func (state *jsVarBroadcastPathSetter) JawsSetPath(_ *jaws.Element, _ string, value any) error {
	state.Value = value
	return nil
}

func TestJsVarSharedBackingStateLocksBroadcastMarshal(t *testing.T) {
	jw, rq := newCoreRequest(t)
	go jw.Serve()

	var mu sync.Mutex
	state := jsVarAliasedValue{}
	jsvar := NewJsVar(&mu, &state)
	elem := rq.NewElement(jsvar)
	if err := jsvar.JawsRender(elem, io.Discard, []any{"v"}); err != nil {
		t.Fatal(err)
	}

	value := &jsVarBlockingJSONValue{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	done := make(chan error, 1)
	go func() {
		done <- jsvar.JawsSetPath(elem, "value", value)
	}()
	awaitJsVarOperation(t, "aliased broadcast marshal start", value.entered)

	if mu.TryLock() {
		mu.Unlock()
		close(value.release)
		t.Fatal("backing lock was released while marshaling an aliased broadcast value")
	}
	close(value.release)
	if err := awaitJsVarOperation(t, "aliased broadcast marshal", done); err != nil {
		t.Fatal(err)
	}
}

func TestJsVarPathSetterRetainedValueLocksBroadcastMarshal(t *testing.T) {
	jw, rq := newCoreRequest(t)
	go jw.Serve()

	var mu sync.Mutex
	state := jsVarBroadcastPathSetter{}
	jsvar := NewJsVar(&mu, &state)
	elem := rq.NewElement(jsvar)
	if err := jsvar.JawsRender(elem, io.Discard, []any{"v"}); err != nil {
		t.Fatal(err)
	}

	value := &jsVarBlockingJSONValue{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	done := make(chan error, 1)
	go func() {
		done <- jsvar.JawsSetPath(elem, "value", value)
	}()
	awaitJsVarOperation(t, "PathSetter broadcast marshal start", value.entered)

	if mu.TryLock() {
		mu.Unlock()
		close(value.release)
		t.Fatal("backing lock was released while marshaling a PathSetter-retained broadcast value")
	}
	close(value.release)
	if err := awaitJsVarOperation(t, "PathSetter broadcast marshal", done); err != nil {
		t.Fatal(err)
	}
}

func TestJsVarGenericBroadcastMarshalFailureReleasesLocks(t *testing.T) {
	tests := []struct {
		name       string
		marshalErr error
		panicValue any
	}{
		{name: "error", marshalErr: errors.New("marshal error")},
		{name: "panic", panicValue: errors.New("marshal panic")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jw, rq := newCoreRequest(t)
			go jw.Serve()

			var mu sync.Mutex
			state := jsVarAliasedValue{}
			jsvar := NewJsVar(&mu, &state)
			elem := rq.NewElement(jsvar)
			if err := jsvar.JawsRender(elem, io.Discard, []any{"v"}); err != nil {
				t.Fatal(err)
			}

			value := &jsVarMarshalOutcomeValue{err: tc.marshalErr, panicValue: tc.panicValue}
			var gotErr error
			var gotPanic any
			func() {
				defer func() { gotPanic = recover() }()
				gotErr = jsvar.JawsSetPath(elem, "value", value)
			}()
			if tc.panicValue == nil {
				if gotPanic != nil || !errors.Is(gotErr, tc.marshalErr) {
					t.Fatalf("marshal result = err %v, panic %v; want error %v", gotErr, gotPanic, tc.marshalErr)
				}
			} else if gotPanic != tc.panicValue {
				t.Fatalf("marshal panic = %v, want %v", gotPanic, tc.panicValue)
			}

			setMuOK := jsvar.setMu.TryLock()
			if setMuOK {
				jsvar.setMu.Unlock()
			}
			muOK := mu.TryLock()
			if muOK {
				mu.Unlock()
			}
			if !setMuOK || !muOK {
				t.Fatalf("marshal failure left locks held: setMu=%t backing=%t", !setMuOK, !muOK)
			}

			if err := jsvar.JawsSetPath(elem, "value", "recovered"); err != nil {
				t.Fatalf("write after marshal failure: %v", err)
			}
			if state.Value != "recovered" {
				t.Fatalf("write after marshal failure left value %#v", state.Value)
			}
		})
	}
}
