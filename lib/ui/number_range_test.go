package ui

import (
	"context"
	"errors"
	"html/template"
	"math"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type numberRangeSource[T comparable] struct {
	mu       sync.Mutex
	value    T
	setCalls int
	selfTag  bool
	tagValue any
}

func newNumberRangeSource[T comparable](value T) *numberRangeSource[T] {
	return &numberRangeSource[T]{value: value, selfTag: true}
}

func (source *numberRangeSource[T]) JawsGet(*jaws.Element) (value T) {
	source.mu.Lock()
	value = source.value
	source.mu.Unlock()
	return
}

func (source *numberRangeSource[T]) JawsSet(_ *jaws.Element, value T) (err error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.setCalls++
	if source.value == value {
		return jaws.ErrValueUnchanged
	}
	source.value = value
	return
}

func (source *numberRangeSource[T]) JawsGetTag() any {
	if source.selfTag {
		return source
	}
	return source.tagValue
}

func (source *numberRangeSource[T]) snapshot() (value T, setCalls int) {
	source.mu.Lock()
	value, setCalls = source.value, source.setCalls
	source.mu.Unlock()
	return
}

func (source *numberRangeSource[T]) store(value T) {
	source.mu.Lock()
	source.value = value
	source.mu.Unlock()
}

type numberRangeGatedSource struct {
	value      atomic.Int32
	setCalls   atomic.Int32
	gateElem   atomic.Pointer[jaws.Element]
	gate       atomic.Bool
	getEntered chan struct{}
	getRelease chan struct{}
	setDone    chan struct{}
}

func newNumberRangeGatedSource(value int8) *numberRangeGatedSource {
	source := &numberRangeGatedSource{
		getEntered: make(chan struct{}),
		getRelease: make(chan struct{}),
		setDone:    make(chan struct{}, 1),
	}
	source.value.Store(int32(value))
	return source
}

func (source *numberRangeGatedSource) JawsGet(elem *jaws.Element) int8 {
	value := int8(source.value.Load())
	if elem == source.gateElem.Load() && source.gate.CompareAndSwap(true, false) {
		close(source.getEntered)
		<-source.getRelease
	}
	return value
}

func (source *numberRangeGatedSource) JawsSet(_ *jaws.Element, value int8) error {
	source.setCalls.Add(1)
	if int8(source.value.Load()) == value {
		return jaws.ErrValueUnchanged
	}
	source.value.Store(int32(value))
	select {
	case source.setDone <- struct{}{}:
	default:
	}
	return nil
}

func (source *numberRangeGatedSource) JawsGetTag() any {
	return source
}

type numberRangeGetter[T comparable] struct {
	value T
}

func (getter *numberRangeGetter[T]) JawsGet(*jaws.Element) T {
	return getter.value
}

type numberRangeErrorSource struct {
	err      error
	setCalls int
}

func (*numberRangeErrorSource) JawsGet(*jaws.Element) int {
	return 7
}

func (source *numberRangeErrorSource) JawsSet(*jaws.Element, int) error {
	source.setCalls++
	return source.err
}

func (source *numberRangeErrorSource) JawsGetTag() any {
	return source
}

type numberRangeGetterView[T comparable] struct {
	source *numberRangeSource[T]
}

func (getter numberRangeGetterView[T]) JawsGet(elem *jaws.Element) T {
	return getter.source.JawsGet(elem)
}

func (getter numberRangeGetterView[T]) JawsGetTag() any {
	return getter.source
}

type numberRangeDecoratedSource struct {
	*numberRangeSource[int]
	consume    bool
	inputCalls int
}

func (source *numberRangeDecoratedSource) JawsInput(*jaws.Element, string) error {
	source.inputCalls++
	if source.consume {
		return nil
	}
	return jaws.ErrEventUnhandled
}

func (*numberRangeDecoratedSource) JawsInitialHTMLAttr(*jaws.Element) template.HTMLAttr {
	return `data-source="yes"`
}

type (
	numberRangeNamedInt     int16
	numberRangeNamedFloat32 float32
)

type numberRangeLogger struct {
	log testErrorLog
}

type numberRangeUI interface {
	jaws.UI
	jaws.InputHandler
}

type numberRangeCountingUI struct {
	numberRangeUI
	updateCalls atomic.Int32
}

func (ui *numberRangeCountingUI) JawsUpdate(elem *jaws.Element) {
	ui.updateCalls.Add(1)
	ui.numberRangeUI.JawsUpdate(elem)
}

func (*numberRangeLogger) Info(string, ...any) {}
func (*numberRangeLogger) Warn(string, ...any) {}
func (logger *numberRangeLogger) Error(_ string, args ...any) {
	logger.log.record(args)
}

func (logger *numberRangeLogger) sync(t *testing.T, jw *jaws.Jaws) []error {
	return logger.log.sync(t, jw)
}

func newNumberRangeLiveRequest(t *testing.T, logger jaws.Logger) *jawstest.TestRequest {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	if logger != nil {
		jw.Logger = logger
	}
	go jw.Serve()
	tr := jawstest.NewTestRequest(jw, nil)
	<-tr.ReadyCh
	t.Cleanup(func() {
		tr.Close()
		select {
		case <-tr.DoneCh:
		case <-time.After(time.Second):
			t.Error("numeric test request did not stop")
		}
		jw.Close()
	})
	return tr
}

func awaitNumberRangeValue(t *testing.T, tr *jawstest.TestRequest, elem *jaws.Element, want string) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-tr.OutCh:
			if msg.What == what.Alert {
				t.Fatalf("unexpected alert while waiting for Value %q: %q", want, msg.Data)
			}
			if msg.Jid == elem.Jid() && msg.What == what.Value {
				if msg.Data != want {
					t.Fatalf("Value update = %q, want %q", msg.Data, want)
				}
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for Value %q", want)
		}
	}
}

func assertNumericValidationPanic(t *testing.T, call func()) {
	t.Helper()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		call()
	}()
	err, ok := recovered.(error)
	if !ok || !strings.Contains(err.Error(), "expected numeric value or numeric bind.Getter") {
		t.Fatalf("panic = %v, want numeric source validation error", recovered)
	}
}

func TestRequestWriterNumericRejectsUnsupportedValues(t *testing.T) {
	_, rq := newCoreRequest(t)
	var rendered strings.Builder
	rw := RequestWriter{Request: rq, Writer: &rendered}
	tests := []struct {
		name string
		call func()
	}{
		{name: "Number nil", call: func() { _ = rw.Number(nil) }},
		{name: "Number unsupported", call: func() { _ = rw.Number("1") }},
		{name: "Range nil", call: func() { _ = rw.Range(nil) }},
		{name: "Range unsupported", call: func() { _ = rw.Range("1") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericValidationPanic(t, tt.call)
		})
	}
}

func TestRequestWriterRangeRendersExplicitBounds(t *testing.T) {
	_, rq := newCoreRequest(t)
	var rendered strings.Builder
	rw := RequestWriter{Request: rq, Writer: &rendered}
	if err := rw.Range(newNumberRangeSource(150.0), `min="0"`, `max="200"`, `step="0.5"`); err != nil {
		t.Fatal(err)
	}
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="range" value="150" min="0" max="200" step="0.5">$`, rendered.String())
}

func TestNumberRangeSourceCapabilities(t *testing.T) {
	t.Run("editable Number marker", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		_, got := renderUI(t, rq, NewNumber(newNumberRangeSource(3)))
		if !strings.Contains(got, " data-jawsnumber") || strings.Contains(got, " readonly") {
			t.Fatalf("editable Number markup = %q", got)
		}
	})

	t.Run("getter-only Number", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		_, got := renderUI(t, rq, NewNumber(&numberRangeGetter[int]{value: 3}))
		if !strings.Contains(got, " readonly") || strings.Contains(got, "data-jawsnumber") {
			t.Fatalf("getter-only Number markup = %q", got)
		}
	})

	t.Run("editable Range", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		_, got := renderUI(t, rq, NewRange(newNumberRangeSource(3)))
		if strings.Contains(got, " disabled") {
			t.Fatalf("editable Range markup = %q", got)
		}
	})

	t.Run("getter-only Range", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		_, got := renderUI(t, rq, NewRange(&numberRangeGetter[int]{value: 3}))
		if !strings.Contains(got, " disabled") {
			t.Fatalf("getter-only Range markup = %q", got)
		}
	})
}

func TestNumberRangeGetterOnlySourcesIgnoreInputAndReceiveUpdates(t *testing.T) {
	tests := []struct {
		name string
		make func(numberRangeGetterView[int]) jaws.UI
	}{
		{name: "Number", make: func(source numberRangeGetterView[int]) jaws.UI { return NewNumber(source) }},
		{name: "Range", make: func(source numberRangeGetterView[int]) jaws.UI { return NewRange(source) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newNumberRangeLiveRequest(t, nil)
			source := newNumberRangeSource(3)
			getter := numberRangeGetterView[int]{source: source}
			widget := tt.make(getter)
			elem := tr.NewElement(widget)
			var rendered strings.Builder
			if err := elem.JawsRender(&rendered, nil); err != nil {
				t.Fatal(err)
			}
			if err := jaws.CallEventHandlers(widget, elem, what.Input, "9"); err != nil {
				t.Fatal(err)
			}
			if value, calls := source.snapshot(); value != 3 || calls != 0 {
				t.Fatalf("getter-only source after input = (%v, %d calls), want (3, 0 calls)", value, calls)
			}

			source.store(4)
			tr.Request.Dirty(source)
			awaitNumberRangeValue(t, tr, elem, "4")
		})
	}
}

func TestNumberRangePreserveNamedAndFloat32Types(t *testing.T) {
	_, rq := newCoreRequest(t)

	integerSource := newNumberRangeSource(numberRangeNamedInt(-12))
	number := NewNumber(integerSource)
	numberElem, got := renderUI(t, rq, number)
	if !strings.Contains(got, `value="-12"`) {
		t.Fatalf("named integer Number markup = %q", got)
	}
	if err := number.JawsInput(numberElem, "120"); err != nil {
		t.Fatal(err)
	}
	if value, calls := integerSource.snapshot(); value != numberRangeNamedInt(120) || calls != 1 {
		t.Fatalf("named integer source = (%v, %d calls), want (120, 1 call)", value, calls)
	}

	floatSource := newNumberRangeSource(numberRangeNamedFloat32(0.1))
	rng := NewRange(floatSource)
	rangeElem, got := renderUI(t, rq, rng)
	if !strings.Contains(got, `value="0.1"`) {
		t.Fatalf("float32 Range markup = %q", got)
	}
	if err := rng.JawsInput(rangeElem, "0.2"); err != nil {
		t.Fatal(err)
	}
	if value, calls := floatSource.snapshot(); value != numberRangeNamedFloat32(0.2) || calls != 1 {
		t.Fatalf("float32 source = (%v, %d calls), want (0.2, 1 call)", value, calls)
	}
}

func TestNumberRangeIntegerRejectionAndCanonicalization(t *testing.T) {
	tr := newNumberRangeLiveRequest(t, nil)
	source := newNumberRangeSource(int8(7))
	number := NewNumber(source)
	elem := tr.NewElement(number)
	var rendered strings.Builder
	if err := elem.JawsRender(&rendered, nil); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		text string
	}{
		{name: "decimal point", text: "7.0"},
		{name: "exponent", text: "7e0"},
		{name: "fractional", text: "7.5"},
		{name: "overflow", text: "128"},
		{name: "empty", text: ""},
		{name: "malformed", text: "bad"},
		{name: "non-finite literal", text: "NaN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr.InCh <- wire.WsMsg{Jid: elem.Jid(), What: what.Input, Data: tt.text}
			awaitNumberRangeValue(t, tr, elem, "7")
			if value, calls := source.snapshot(); value != 7 || calls != 0 {
				t.Fatalf("Number source after %q = (%v, %d calls), want (7, 0 calls)", tt.text, value, calls)
			}
		})
	}
	tr.InCh <- wire.WsMsg{Jid: elem.Jid(), What: what.Input, Data: "08"}
	awaitNumberRangeValue(t, tr, elem, "8")
	if value, calls := source.snapshot(); value != 8 || calls != 1 {
		t.Fatalf("Number source after accepted canonicalization = (%v, %d calls), want (8, 1 call)", value, calls)
	}

	_, rq := newCoreRequest(t)
	rangeSource := newNumberRangeSource(int8(7))
	rng := NewRange(rangeSource)
	rangeElem, _ := renderUI(t, rq, rng)
	for _, text := range []string{"7.0", "7e0", "7.5", "128"} {
		if err := rng.JawsInput(rangeElem, text); err != nil {
			t.Fatalf("Range.JawsInput(%q): %v", text, err)
		}
	}
	if value, calls := rangeSource.snapshot(); value != 7 || calls != 0 {
		t.Fatalf("Range source after rejected input = (%v, %d calls), want (7, 0 calls)", value, calls)
	}
}

func TestNumberRangeInputSetterError(t *testing.T) {
	tests := []struct {
		name   string
		widget func(*numberRangeErrorSource) numberRangeUI
	}{
		{name: "Number", widget: func(source *numberRangeErrorSource) numberRangeUI { return NewNumber(source) }},
		{name: "Range", widget: func(source *numberRangeErrorSource) numberRangeUI { return NewRange(source) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newCoreRequest(t)
			setErr := errors.New("setter error")
			source := &numberRangeErrorSource{err: setErr}
			widget := tt.widget(source)
			elem, _ := renderUI(t, rq, widget)

			if err := widget.JawsInput(elem, "8"); !errors.Is(err, setErr) {
				t.Fatalf("JawsInput error = %v, want setter error", err)
			}
			if cause := context.Cause(rq.Context()); cause != nil {
				t.Fatalf("request cancellation cause = %v, want nil", cause)
			}
			if source.setCalls != 1 {
				t.Fatalf("setter called %d times, want 1", source.setCalls)
			}
		})
	}
}

func TestRangeBroadcastsCanonicalValueAcrossRequests(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	first := jawstest.NewTestRequest(jw, nil)
	second := jawstest.NewTestRequest(jw, nil)
	<-first.ReadyCh
	<-second.ReadyCh
	defer func() {
		first.Close()
		second.Close()
		<-first.DoneCh
		<-second.DoneCh
		jw.Close()
	}()

	source := newNumberRangeSource(numberRangeNamedInt(1))
	firstRange := NewRange(source)
	secondRange := NewRange(source)
	firstElem := first.NewElement(firstRange)
	secondElem := second.NewElement(secondRange)
	for _, elem := range []*jaws.Element{firstElem, secondElem} {
		var output strings.Builder
		if err = elem.JawsRender(&output, nil); err != nil {
			t.Fatal(err)
		}
	}

	if err = firstRange.JawsInput(firstElem, "02"); err != nil {
		t.Fatal(err)
	}
	awaitNumberRangeValue(t, first, firstElem, "2")
	awaitNumberRangeValue(t, second, secondElem, "2")
	if value, calls := source.snapshot(); value != 2 || calls != 1 {
		t.Fatalf("shared Range source = (%v, %d calls), want (2, 1 call)", value, calls)
	}
}

func TestNumericReconciliationTargetsOnlyOrigin(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantValue    string
		wantSetCalls int
		widget       func(*numberRangeSource[int8]) numberRangeUI
	}{
		{name: "Range invalid", text: "7.5", wantValue: "7", widget: func(source *numberRangeSource[int8]) numberRangeUI { return NewRange(source) }},
		{name: "Range unchanged", text: "07", wantValue: "7", wantSetCalls: 1, widget: func(source *numberRangeSource[int8]) numberRangeUI { return NewRange(source) }},
		{name: "Number invalid", text: "7.5", wantValue: "7", widget: func(source *numberRangeSource[int8]) numberRangeUI { return NewNumber(source) }},
		{name: "Number unchanged", text: "7", wantSetCalls: 1, widget: func(source *numberRangeSource[int8]) numberRangeUI { return NewNumber(source) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				jw, err := jaws.New()
				if err != nil {
					t.Fatal(err)
				}
				go jw.Serve()
				first := jawstest.NewTestRequest(jw, nil)
				second := jawstest.NewTestRequest(jw, nil)
				<-first.ReadyCh
				<-second.ReadyCh
				defer func() {
					first.Close()
					second.Close()
					jw.Close()
					synctest.Wait()
				}()

				source := newNumberRangeSource(int8(7))
				origin := &numberRangeCountingUI{numberRangeUI: tt.widget(source)}
				firstPeer := &numberRangeCountingUI{numberRangeUI: tt.widget(source)}
				secondPeer := &numberRangeCountingUI{numberRangeUI: tt.widget(source)}

				originElem := first.NewElement(origin)
				var rendered strings.Builder
				if err = originElem.JawsRender(&rendered, nil); err != nil {
					t.Fatal(err)
				}
				for _, item := range []struct {
					request *jawstest.TestRequest
					ui      *numberRangeCountingUI
				}{
					{request: first, ui: firstPeer},
					{request: second, ui: secondPeer},
				} {
					elem := item.request.NewElement(item.ui)
					rendered.Reset()
					if err = elem.JawsRender(&rendered, nil); err != nil {
						t.Fatal(err)
					}
				}
				if got := first.GetElements(source); len(got) != 2 {
					t.Fatalf("origin request source Elements = %d, want 2", len(got))
				}
				if got := second.GetElements(source); len(got) != 1 {
					t.Fatalf("peer request source Elements = %d, want 1", len(got))
				}

				first.InCh <- wire.WsMsg{Jid: originElem.Jid(), What: what.Input, Data: tt.text}
				synctest.Wait()
				// Numeric correction uses the ordinary batched dirty pass.
				time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
				synctest.Wait()

				select {
				case msg := <-first.OutCh:
					if tt.wantValue == "" {
						t.Fatalf("canonical numeric input produced output %#v", msg)
					}
					if msg.Jid != originElem.Jid() || msg.What != what.Value || msg.Data != tt.wantValue {
						t.Fatalf("numeric correction = %#v, want Value %q for %v", msg, tt.wantValue, originElem.Jid())
					}
				default:
					if tt.wantValue != "" {
						t.Fatal("numeric input did not produce a correction")
					}
				}
				for _, item := range []struct {
					name    string
					request *jawstest.TestRequest
				}{
					{name: "origin request", request: first},
					{name: "peer request", request: second},
				} {
					select {
					case msg := <-item.request.OutCh:
						t.Fatalf("unexpected output on %s: %#v", item.name, msg)
					default:
					}
				}
				if value, calls := source.snapshot(); value != 7 || calls != tt.wantSetCalls {
					t.Fatalf("numeric source = (%v, %d calls), want (7, %d calls)", value, calls, tt.wantSetCalls)
				}

				for _, item := range []struct {
					name  string
					ui    *numberRangeCountingUI
					calls int32
				}{
					{name: "origin", ui: origin, calls: 1},
					{name: "same-request peer", ui: firstPeer},
					{name: "other-request peer", ui: secondPeer},
				} {
					if calls := item.ui.updateCalls.Load(); calls != item.calls {
						t.Fatalf("%s JawsUpdate calls = %d, want %d", item.name, calls, item.calls)
					}
				}
			})
		})
	}
}

func TestNumericRejectedCorrectionConvergesToNewerSource(t *testing.T) {
	tests := []struct {
		name   string
		widget func(*numberRangeGatedSource) jaws.UI
	}{
		{name: "Number", widget: func(source *numberRangeGatedSource) jaws.UI { return NewNumber(source) }},
		{name: "Range", widget: func(source *numberRangeGatedSource) jaws.UI { return NewRange(source) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				jw, err := jaws.New()
				if err != nil {
					t.Fatal(err)
				}
				go jw.Serve()
				originRequest := jawstest.NewTestRequest(jw, nil)
				peerRequest := jawstest.NewTestRequest(jw, nil)
				<-originRequest.ReadyCh
				<-peerRequest.ReadyCh
				defer func() {
					originRequest.Close()
					peerRequest.Close()
					jw.Close()
					synctest.Wait()
				}()

				source := newNumberRangeGatedSource(7)
				releaseGetter := sync.OnceFunc(func() { close(source.getRelease) })
				defer releaseGetter()
				originElem := originRequest.NewElement(tt.widget(source))
				peerElem := peerRequest.NewElement(tt.widget(source))
				var rendered strings.Builder
				if err = originElem.JawsRender(&rendered, nil); err != nil {
					t.Fatal(err)
				}
				rendered.Reset()
				if err = peerElem.JawsRender(&rendered, nil); err != nil {
					t.Fatal(err)
				}
				source.gateElem.Store(originElem)
				source.gate.Store(true)

				originRequest.InCh <- wire.WsMsg{Jid: originElem.Jid(), What: what.Input, Data: "7.5"}
				select {
				case <-source.getEntered:
				case <-time.After(time.Second):
					t.Fatal("timed out waiting for gated canonical read")
				}
				peerRequest.InCh <- wire.WsMsg{Jid: peerElem.Jid(), What: what.Input, Data: "8"}
				select {
				case <-source.setDone:
				case <-time.After(time.Second):
					t.Fatal("timed out waiting for peer setter")
				}
				// Let the accepted peer event finish dirtying the shared source before
				// advancing the JaWS update tick.
				synctest.Wait()
				time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
				synctest.Wait()
				releaseGetter()
				synctest.Wait()

				var values []string
			drainOrigin:
				for {
					select {
					case msg := <-originRequest.OutCh:
						if msg.Jid != originElem.Jid() || msg.What != what.Value {
							t.Fatalf("unexpected origin output %#v", msg)
						}
						values = append(values, msg.Data)
					default:
						break drainOrigin
					}
				}
				if !reflect.DeepEqual(values, []string{"7", "8"}) {
					t.Fatalf("origin Value updates = %q, want [7 8]", values)
				}
				var peerValues []string
			drainPeer:
				for {
					select {
					case msg := <-peerRequest.OutCh:
						if msg.Jid != peerElem.Jid() || msg.What != what.Value {
							t.Fatalf("unexpected peer output %#v", msg)
						}
						peerValues = append(peerValues, msg.Data)
					default:
						break drainPeer
					}
				}
				if len(peerValues) != 0 {
					t.Fatalf("peer Value updates = %q, want none", peerValues)
				}
				if value, calls := int8(source.value.Load()), source.setCalls.Load(); value != 8 || calls != 1 {
					t.Fatalf("source = (%v, %d calls), want (8, 1 call)", value, calls)
				}
			})
		})
	}
}

func TestNumberRangeCancelOnBoundNonFiniteValues(t *testing.T) {
	t.Run("Number render", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		source := newNumberRangeSource(numberRangeNamedFloat32(float32(math.NaN())))
		elem := rq.NewElement(NewNumber(source))
		var rendered strings.Builder
		if err := elem.JawsRender(&rendered, nil); err != nil {
			t.Fatal(err)
		}
		if cause := context.Cause(rq.Context()); !errors.Is(cause, jaws.ErrValueNotFinite) {
			t.Fatalf("cancel cause = %v, want ErrValueNotFinite", cause)
		}
	})

	t.Run("Range render", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		source := newNumberRangeSource(math.Inf(1))
		elem := rq.NewElement(NewRange(source))
		var rendered strings.Builder
		if err := elem.JawsRender(&rendered, nil); err != nil {
			t.Fatal(err)
		}
		if cause := context.Cause(rq.Context()); !errors.Is(cause, jaws.ErrValueNotFinite) {
			t.Fatalf("cancel cause = %v, want ErrValueNotFinite", cause)
		}
	})

	t.Run("Number update", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		source := newNumberRangeSource(float32(1))
		number := NewNumber(source)
		elem, _ := renderUI(t, rq, number)
		source.store(float32(math.Inf(1)))
		number.JawsUpdate(elem)
		if cause := context.Cause(rq.Context()); !errors.Is(cause, jaws.ErrValueNotFinite) {
			t.Fatalf("cancel cause = %v, want ErrValueNotFinite", cause)
		}
	})

	t.Run("Range update", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		source := newNumberRangeSource(float64(1))
		rng := NewRange(source)
		elem, _ := renderUI(t, rq, rng)
		source.store(math.NaN())
		rng.JawsUpdate(elem)
		if cause := context.Cause(rq.Context()); !errors.Is(cause, jaws.ErrValueNotFinite) {
			t.Fatalf("cancel cause = %v, want ErrValueNotFinite", cause)
		}
	})
}

func TestNumberRangeRequireEditableSourceTags(t *testing.T) {
	tests := []struct {
		name       string
		tagValue   any
		wantSecond error
	}{
		{name: "nil", tagValue: nil},
		{name: "empty", tagValue: []any{}},
		{name: "illegal", tagValue: "count", wantSecond: tag.ErrIllegalTagType},
		{name: "noncomparable", tagValue: []int{1}},
	}
	widgets := []struct {
		name string
		make func(*numberRangeSource[int]) jaws.UI
	}{
		{name: "Number", make: func(source *numberRangeSource[int]) jaws.UI { return NewNumber(source) }},
		{name: "Range", make: func(source *numberRangeSource[int]) jaws.UI { return NewRange(source) }},
	}
	for _, widget := range widgets {
		for _, tt := range tests {
			t.Run(widget.name+"/"+tt.name, func(t *testing.T) {
				_, rq := newCoreRequest(t)
				source := newNumberRangeSource(1)
				source.selfTag = false
				source.tagValue = tt.tagValue
				elem := rq.NewElement(widget.make(source))
				var rendered strings.Builder
				err := elem.JawsRender(&rendered, nil)
				if !errors.Is(err, tag.ErrNotUsableAsTag) {
					t.Fatalf("render error = %v, want ErrNotUsableAsTag", err)
				}
				if tt.wantSecond != nil && !errors.Is(err, tt.wantSecond) {
					t.Fatalf("render error = %v, want additional match for %v", err, tt.wantSecond)
				}
			})
		}
	}
}

func TestNumberRangePreserveSourceHandlerAndInitialAttribute(t *testing.T) {
	widgets := []struct {
		name string
		make func(*numberRangeDecoratedSource) jaws.UI
	}{
		{name: "Number", make: func(source *numberRangeDecoratedSource) jaws.UI { return NewNumber(source) }},
		{name: "Range", make: func(source *numberRangeDecoratedSource) jaws.UI { return NewRange(source) }},
	}
	for _, widget := range widgets {
		t.Run(widget.name, func(t *testing.T) {
			_, rq := newCoreRequest(t)
			source := &numberRangeDecoratedSource{
				numberRangeSource: newNumberRangeSource(4),
				consume:           true,
			}
			input := widget.make(source)
			elem, got := renderUI(t, rq, input)
			if !strings.Contains(got, `data-source="yes"`) {
				t.Fatalf("markup omitted source attribute: %q", got)
			}

			if err := jaws.CallEventHandlers(input, elem, what.Input, "9"); err != nil {
				t.Fatal(err)
			}
			if value, calls := source.snapshot(); value != 4 || calls != 0 || source.inputCalls != 1 {
				t.Fatalf("consumed input left source = (%v, %d sets, %d handler calls)", value, calls, source.inputCalls)
			}

			source.consume = false
			if err := jaws.CallEventHandlers(input, elem, what.Input, "10"); err != nil {
				t.Fatal(err)
			}
			if value, calls := source.snapshot(); value != 10 || calls != 1 || source.inputCalls != 2 {
				t.Fatalf("fallback input left source = (%v, %d sets, %d handler calls)", value, calls, source.inputCalls)
			}
		})
	}
}

func TestNumericRegisterLogsAndQueuesNothing(t *testing.T) {
	tests := []struct {
		name   string
		widget func() numberRangeUI
	}{
		{name: "Number", widget: func() numberRangeUI { return NewNumber(newNumberRangeSource(3)) }},
		{name: "Range", widget: func() numberRangeUI { return NewRange(newNumberRangeSource(3)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				jw, err := jaws.New()
				if err != nil {
					t.Fatal(err)
				}
				logger := new(numberRangeLogger)
				jw.Logger = logger
				go jw.Serve()
				tr := jawstest.NewTestRequest(jw, nil)
				<-tr.ReadyCh
				defer func() {
					tr.Close()
					jw.Close()
					synctest.Wait()
				}()

				widget := tt.widget()
				(RequestWriter{Request: tr.Request, Writer: tr.Recorder}).Register(widget)
				logged := logger.sync(t, jw)
				if len(logged) != 1 || !strings.Contains(logged[0].Error(), "before successful rendering") {
					t.Fatalf("logged errors = %v", logged)
				}

				tr.InCh <- wire.WsMsg{}
				synctest.Wait()
				select {
				case msg := <-tr.OutCh:
					t.Fatalf("update before render queued outbound message %#v", msg)
				default:
				}
			})
		})
	}
}

func TestRequestWriterNumericReflection(t *testing.T) {
	_, rq := newCoreRequest(t)
	static := numberRangeNamedInt(-5)
	getter := &numberRangeGetter[numberRangeNamedInt]{value: 6}
	setter := newNumberRangeSource(numberRangeNamedInt(7))
	data := struct {
		Static numberRangeNamedInt
		Getter *numberRangeGetter[numberRangeNamedInt]
		Setter *numberRangeSource[numberRangeNamedInt]
	}{
		Static: static,
		Getter: getter,
		Setter: setter,
	}

	var rendered strings.Builder
	rw := RequestWriter{Request: rq, Writer: &rendered}
	page := template.Must(template.New("numeric").Parse(
		`{{$.Number .Dot.Static}}{{$.Range .Dot.Static}}` +
			`{{$.Number .Dot.Getter}}{{$.Number .Dot.Setter}}` +
			`{{$.Range .Dot.Setter}}`,
	))
	if err := page.Execute(&rendered, With{RequestWriter: rw, Dot: data}); err != nil {
		t.Fatal(err)
	}
	got := rendered.String()
	if strings.Count(got, `<input `) != 5 ||
		strings.Count(got, " readonly") != 2 ||
		strings.Count(got, " disabled") != 1 ||
		strings.Count(got, " data-jawsnumber") != 1 {
		t.Fatalf("numeric template markup = %q", got)
	}
	for _, value := range []string{"-5", "6", "7"} {
		if !strings.Contains(got, `value="`+value+`"`) {
			t.Fatalf("numeric template markup omitted value %q: %q", value, got)
		}
	}
	if elems := rq.GetElements(setter); len(elems) != 2 {
		t.Fatalf("template setter registered %d Elements, want 2", len(elems))
	}
}
