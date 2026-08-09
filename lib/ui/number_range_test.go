package ui

import (
	"context"
	"errors"
	"html/template"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
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

type numberRangeGetter[T comparable] struct {
	value T
}

func (getter *numberRangeGetter[T]) JawsGet(*jaws.Element) T {
	return getter.value
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

type numberRangeCustomValue struct {
	Units int
}

type numberRangeCustomCodec struct{}

func (numberRangeCustomCodec) FormatNumber(value numberRangeCustomValue) string {
	return strconv.Itoa(value.Units)
}

func (numberRangeCustomCodec) ParseNumber(text string) (value numberRangeCustomValue, ok bool) {
	var err error
	value.Units, err = strconv.Atoi(text)
	ok = err == nil
	return
}

type numberRangeSwitchCodec struct {
	invalid atomic.Bool
}

func (codec *numberRangeSwitchCodec) FormatNumber(value int) string {
	if codec.invalid.Load() {
		return "+" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func (*numberRangeSwitchCodec) ParseNumber(text string) (value int, ok bool) {
	var err error
	value, err = strconv.Atoi(text)
	ok = err == nil
	return
}

type numberRangeLogger struct {
	mu     sync.Mutex
	errors []error
}

type numberRangeProbeUpdater struct{}

func (*numberRangeProbeUpdater) JawsUpdate(*jaws.Element) {}

func (*numberRangeLogger) Info(string, ...any) {}
func (*numberRangeLogger) Warn(string, ...any) {}
func (logger *numberRangeLogger) Error(_ string, args ...any) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			if err, ok := args[i+1].(error); ok {
				logger.errors = append(logger.errors, err)
			}
		}
	}
}

func (logger *numberRangeLogger) snapshot() []error {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	return append([]error(nil), logger.errors...)
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
	if err := number.JawsInput(numberElem, "1.2e2"); err != nil {
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
		name      string
		text      string
		wantCalls int
	}{
		{name: "unchanged alternate spelling", text: "7.0", wantCalls: 1},
		{name: "fractional", text: "7.5", wantCalls: 1},
		{name: "overflow", text: "128", wantCalls: 1},
		{name: "empty", text: "", wantCalls: 1},
		{name: "malformed", text: "bad", wantCalls: 1},
		{name: "non-finite literal", text: "NaN", wantCalls: 1},
		{name: "non-finite overflow", text: "1e999", wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := number.JawsInput(elem, tt.text); err != nil {
				t.Fatalf("Number.JawsInput(%q): %v", tt.text, err)
			}
			if value, calls := source.snapshot(); value != 7 || calls != tt.wantCalls {
				t.Fatalf("Number source after %q = (%v, %d calls), want (7, %d calls)", tt.text, value, calls, tt.wantCalls)
			}
			awaitNumberRangeValue(t, tr, elem, "7")
		})
	}

	_, rq := newCoreRequest(t)
	rangeSource := newNumberRangeSource(int8(7))
	rng := NewRange(rangeSource)
	rangeElem, _ := renderUI(t, rq, rng)
	for _, text := range []string{"7.5", "128"} {
		if err := rng.JawsInput(rangeElem, text); err != nil {
			t.Fatalf("Range.JawsInput(%q): %v", text, err)
		}
	}
	if value, calls := rangeSource.snapshot(); value != 7 || calls != 0 {
		t.Fatalf("Range source after rejected input = (%v, %d calls), want (7, 0 calls)", value, calls)
	}
}

func TestRangeInputErrorClassification(t *testing.T) {
	t.Run("parse contract error cancels request", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		source := newNumberRangeSource(7)
		rng := NewRange(source)
		elem, _ := renderUI(t, rq, rng)
		parseErr := errors.New("parse contract error")
		rng.binding.parseValue = func(string) (any, bool, error) {
			return nil, false, parseErr
		}

		if err := rng.JawsInput(elem, "8"); err != nil {
			t.Fatalf("JawsInput error = %v, want nil", err)
		}
		if cause := context.Cause(rq.Context()); !errors.Is(cause, parseErr) {
			t.Fatalf("request cancellation cause = %v, want parse error", cause)
		}
		if value, calls := source.snapshot(); value != 7 || calls != 0 {
			t.Fatalf("Range source = (%v, %d calls), want (7, 0 calls)", value, calls)
		}
	})

	t.Run("setter error propagates", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		source := newNumberRangeSource(7)
		rng := NewRange(source)
		elem, _ := renderUI(t, rq, rng)
		setErr := errors.New("setter error")
		rng.binding.setValue = func(*jaws.Element, any) error {
			return setErr
		}

		if err := rng.JawsInput(elem, "8"); !errors.Is(err, setErr) {
			t.Fatalf("JawsInput error = %v, want setter error", err)
		}
		if cause := context.Cause(rq.Context()); cause != nil {
			t.Fatalf("request cancellation cause = %v, want nil", cause)
		}
		if value, calls := source.snapshot(); value != 7 || calls != 0 {
			t.Fatalf("Range source = (%v, %d calls), want (7, 0 calls)", value, calls)
		}
	})
}

func TestRangeBroadcastsExactValueAcrossRequests(t *testing.T) {
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

	if err = firstRange.JawsInput(firstElem, "2"); err != nil {
		t.Fatal(err)
	}
	awaitNumberRangeValue(t, second, secondElem, "2")
	if value, calls := source.snapshot(); value != 2 || calls != 1 {
		t.Fatalf("shared Range source = (%v, %d calls), want (2, 1 call)", value, calls)
	}
}

func TestRangeRejectedInputRestoresCanonicalValue(t *testing.T) {
	const probe = "range-rejection-event-barrier"
	logger := new(numberRangeLogger)
	tr := newNumberRangeLiveRequest(t, logger)
	source := newNumberRangeSource(int8(7))
	rng := NewRange(source)
	elem := tr.NewElement(rng)
	var rendered strings.Builder
	if err := elem.JawsRender(&rendered, nil); err != nil {
		t.Fatal(err)
	}

	probeErr := errors.New(probe)
	var probeAlert wire.WsMsg
	probeAlert.FillAlert(probeErr)
	rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
	probeID := rw.Register(new(numberRangeProbeUpdater), jaws.InputFn(func(*jaws.Element, string) error {
		return probeErr
	}))
	// The single event caller handles these in order. Its sentinel Alert is therefore
	// enqueued after any Alerts mistakenly produced by the rejected Range inputs.
	for _, text := range []string{"7.5", "8.5", "9.5"} {
		tr.InCh <- wire.WsMsg{Jid: elem.Jid(), What: what.Input, Data: text}
	}
	tr.InCh <- wire.WsMsg{Jid: probeID, What: what.Input}
	sawCanonical := false
	sawBarrier := false
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-tr.OutCh:
			switch msg.What {
			case what.Value:
				if msg.Jid == elem.Jid() && msg.Data == "7" {
					sawCanonical = true
				}
			case what.Alert:
				if msg.Data != probeAlert.Data {
					t.Fatalf("rejected Range input produced Alert %q", msg.Data)
				}
				sawBarrier = true
			}
			if sawCanonical && sawBarrier {
				if value, calls := source.snapshot(); value != 7 || calls != 0 {
					t.Fatalf("Range source = (%v, %d calls), want (7, 0 calls)", value, calls)
				}
				return
			}
		case <-timer.C:
			t.Fatal("timed out draining rejected Range output")
		}
	}
}

func TestRangeRegisterSendsInitialValue(t *testing.T) {
	tr := newNumberRangeLiveRequest(t, nil)
	rng := NewRange(newNumberRangeSource(numberRangeNamedInt(0)))
	rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
	id := rw.Register(rng)

	tr.InCh <- wire.WsMsg{}
	select {
	case msg := <-tr.OutCh:
		if msg.What != what.Value || msg.Jid != id || msg.Data != "0" {
			t.Fatalf("initial Range update = {%v %v %q}, want {%v %v %q}", msg.What, msg.Jid, msg.Data, what.Value, id, "0")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial registered Range value")
	}
}

func TestNumberCodecFormatErrorsAtWidgetBoundary(t *testing.T) {
	t.Run("render returns error", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		codec := new(numberRangeSwitchCodec)
		codec.invalid.Store(true)
		number := NewNumberWith(newNumberRangeSource(3), codec)
		elem := rq.NewElement(number)
		var rendered strings.Builder
		if err := elem.JawsRender(&rendered, nil); !errors.Is(err, ErrNumberFormat) {
			t.Fatalf("render error = %v, want ErrNumberFormat", err)
		}
		if cause := context.Cause(rq.Context()); cause != nil {
			t.Fatalf("render cancellation cause = %v, want nil", cause)
		}
	})

	t.Run("input cancels request", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		codec := new(numberRangeSwitchCodec)
		source := newNumberRangeSource(3)
		number := NewNumberWith(source, codec)
		elem, _ := renderUI(t, rq, number)
		codec.invalid.Store(true)
		if err := number.JawsInput(elem, "4"); err != nil {
			t.Fatalf("input error = %v, want nil", err)
		}
		if cause := context.Cause(rq.Context()); !errors.Is(cause, ErrNumberFormat) {
			t.Fatalf("input cancellation cause = %v, want ErrNumberFormat", cause)
		}
		if value, calls := source.snapshot(); value != 3 || calls != 0 {
			t.Fatalf("Number source = (%v, %d calls), want (3, 0 calls)", value, calls)
		}
	})

	t.Run("update cancels request", func(t *testing.T) {
		_, rq := newCoreRequest(t)
		codec := new(numberRangeSwitchCodec)
		number := NewNumberWith(newNumberRangeSource(3), codec)
		elem, _ := renderUI(t, rq, number)
		codec.invalid.Store(true)
		number.JawsUpdate(elem)
		if cause := context.Cause(rq.Context()); !errors.Is(cause, ErrNumberFormat) {
			t.Fatalf("update cancellation cause = %v, want ErrNumberFormat", cause)
		}
	})
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

func TestNumberUpdateBeforeRenderLogsAndQueuesNothing(t *testing.T) {
	logger := new(numberRangeLogger)
	tr := newNumberRangeLiveRequest(t, logger)
	number := NewNumber(newNumberRangeSource(3))
	elem := tr.NewElement(number)

	number.JawsUpdate(elem)
	logged := logger.snapshot()
	if len(logged) != 1 || !strings.Contains(logged[0].Error(), "before successful rendering") {
		t.Fatalf("logged errors = %v", logged)
	}

	tr.InCh <- wire.WsMsg{}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case msg := <-tr.OutCh:
		t.Fatalf("update before render queued outbound message %#v", msg)
	case <-timer.C:
	}
}

func TestRequestWriterNumericReflectionAndCustomTemplateBinding(t *testing.T) {
	_, rq := newCoreRequest(t)
	static := numberRangeNamedInt(-5)
	getter := &numberRangeGetter[numberRangeNamedInt]{value: 6}
	setter := newNumberRangeSource(numberRangeNamedInt(7))
	customSource := newNumberRangeSource(numberRangeCustomValue{Units: 8})
	custom := NewNumericBinding(customSource, numberRangeCustomCodec{})
	data := struct {
		Static numberRangeNamedInt
		Getter *numberRangeGetter[numberRangeNamedInt]
		Setter *numberRangeSource[numberRangeNamedInt]
		Custom *NumericBinding
	}{
		Static: static,
		Getter: getter,
		Setter: setter,
		Custom: custom,
	}

	var rendered strings.Builder
	rw := RequestWriter{Request: rq, Writer: &rendered}
	page := template.Must(template.New("numeric").Parse(
		`{{$.Number .Dot.Static}}{{$.Range .Dot.Static}}` +
			`{{$.Number .Dot.Getter}}{{$.Number .Dot.Setter}}` +
			`{{$.Range .Dot.Setter}}{{$.Number .Dot.Custom}}`,
	))
	if err := page.Execute(&rendered, With{RequestWriter: rw, Dot: data}); err != nil {
		t.Fatal(err)
	}
	got := rendered.String()
	if strings.Count(got, `<input `) != 6 ||
		strings.Count(got, " readonly") != 2 ||
		strings.Count(got, " disabled") != 1 ||
		strings.Count(got, " data-jawsnumber") != 2 {
		t.Fatalf("numeric template markup = %q", got)
	}
	for _, value := range []string{"-5", "6", "7", "8"} {
		if !strings.Contains(got, `value="`+value+`"`) {
			t.Fatalf("numeric template markup omitted value %q: %q", value, got)
		}
	}

	elems := rq.GetElements(customSource)
	if len(elems) != 1 {
		t.Fatalf("custom source registered %d Elements, want 1", len(elems))
	}
	number, ok := elems[0].UI().(*Number)
	if !ok {
		t.Fatalf("custom source UI = %T, want *Number", elems[0].UI())
	}
	if err := number.JawsInput(elems[0], "12"); err != nil {
		t.Fatal(err)
	}
	if value, calls := customSource.snapshot(); value != (numberRangeCustomValue{Units: 12}) || calls != 1 {
		t.Fatalf("custom source = (%v, %d calls), want ({12}, 1 call)", value, calls)
	}
}
