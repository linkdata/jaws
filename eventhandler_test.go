package jaws

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type testJawsEvent struct {
	msgCh    chan string
	tagValue any
	clickerr error
	inputerr error
}

func (t *testJawsEvent) JawsClick(elem *Element, click Click) (err error) {
	if err = t.clickerr; err == nil {
		t.msgCh <- fmt.Sprintf("JawsClick: %q", click.Name)
	}
	return
}

func (t *testJawsEvent) JawsInput(elem *Element, value string) (err error) {
	if err = t.inputerr; err == nil {
		t.msgCh <- fmt.Sprintf("JawsInput: %q", value)
	} else {
		t.msgCh <- err.Error()
	}
	return
}

func (t *testJawsEvent) JawsGetTag(tag.Context) (tagValue any) {
	return t.tagValue
}

func (t *testJawsEvent) JawsRender(elem *Element, w io.Writer, params []any) (err error) {
	var tagValue any
	if tagValue, _, err = elem.ApplyGetter(t); err == nil {
		_, _ = w.Write([]byte(fmt.Sprint(params)))
		t.msgCh <- fmt.Sprintf("JawsRender(%d)%#v", elem.jid, tagValue)
	}
	return
}

func (t *testJawsEvent) JawsUpdate(elem *Element) {
	t.msgCh <- fmt.Sprintf("JawsUpdate(%d)", elem.jid)
}

var _ ClickHandler = (*testJawsEvent)(nil)
var _ InputHandler = (*testJawsEvent)(nil)
var _ tag.TagGetter = (*testJawsEvent)(nil)
var _ UI = (*testJawsEvent)(nil)

func Test_JawsInput_InvokesJawsInputForDualHandler(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}
	zomgItem := &testUi{}
	id := rq.Register(zomgItem, je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	rq.InCh <- wire.WsMsg{Data: "typed", Jid: id, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != `JawsInput: "typed"` {
			t.Errorf("unexpected handler call: %q", s)
		}
	}
}

type testJawsInputHandler struct {
	UI
	msgCh    chan string
	inputerr error
}

func (t *testJawsInputHandler) JawsGetHTML(elem *Element) template.HTML {
	return "tjIH"
}

func (t *testJawsInputHandler) JawsInput(elem *Element, value string) (err error) {
	if err = t.inputerr; err == nil {
		t.msgCh <- fmt.Sprintf("JawsInput: %q", value)
	} else {
		t.msgCh <- err.Error()
	}
	return
}

type testPanicInputHandler struct {
	panicVal any
}

func (h testPanicInputHandler) JawsInput(elem *Element, value string) error {
	panic(h.panicVal)
}

type testClickCounter struct {
	n         int
	wantName  string
	lastValue Click
}

func (c *testClickCounter) JawsClick(elem *Element, click Click) error {
	c.lastValue = click
	if click.Name != c.wantName {
		return ErrEventUnhandled
	}
	c.n++
	return nil
}

type testContextMenuCounter struct {
	n         int
	wantName  string
	lastValue Click
}

func (c *testContextMenuCounter) JawsContextMenu(elem *Element, click Click) error {
	c.lastValue = click
	if click.Name != c.wantName {
		return ErrEventUnhandled
	}
	c.n++
	return nil
}

type testPointerCounter struct {
	n         int
	wantName  string
	lastValue Pointer
}

func (c *testPointerCounter) JawsPointer(elem *Element, ptr Pointer) error {
	c.lastValue = ptr
	if ptr.Name != c.wantName {
		return ErrEventUnhandled
	}
	c.n++
	return nil
}

type clickInputSetRecorder struct {
	clickRet     error
	pointerRet   error
	inputRet     error
	clickCalls   int
	pointerCalls int
	inputCalls   int
}

type clickOnlyComboHandler struct{ rec *clickInputSetRecorder }

func (h clickOnlyComboHandler) JawsClick(elem *Element, click Click) error {
	h.rec.clickCalls++
	return h.rec.clickRet
}

type pointerOnlyComboHandler struct{ rec *clickInputSetRecorder }

func (h pointerOnlyComboHandler) JawsPointer(elem *Element, ptr Pointer) error {
	h.rec.pointerCalls++
	return h.rec.pointerRet
}

type inputOnlyComboHandler struct{ rec *clickInputSetRecorder }

func (h inputOnlyComboHandler) JawsInput(elem *Element, value string) error {
	h.rec.inputCalls++
	return h.rec.inputRet
}

type dualClickInputComboHandler struct{ rec *clickInputSetRecorder }

func (h dualClickInputComboHandler) JawsClick(elem *Element, click Click) error {
	h.rec.clickCalls++
	return h.rec.clickRet
}

func (h dualClickInputComboHandler) JawsInput(elem *Element, value string) error {
	h.rec.inputCalls++
	return h.rec.inputRet
}

func Test_CallEventHandlers_ClickDispatchCombinations(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	elem := rq.NewElement(testDivWidget{inner: "x"})
	wrappedUnhandled := fmt.Errorf("wrapped: %w", ErrEventUnhandled)

	tests := []struct {
		name       string
		make       func(*clickInputSetRecorder) any
		clickRet   error
		inputRet   error
		wantErr    error
		wantClicks int
		wantInputs int
	}{
		{
			name:       "click-only returns nil",
			make:       func(rec *clickInputSetRecorder) any { return clickOnlyComboHandler{rec: rec} },
			wantErr:    nil,
			wantClicks: 1,
			wantInputs: 0,
		},
		{
			name:       "click-only returns ErrEventUnhandled",
			make:       func(rec *clickInputSetRecorder) any { return clickOnlyComboHandler{rec: rec} },
			clickRet:   ErrEventUnhandled,
			wantErr:    ErrEventUnhandled,
			wantClicks: 1,
			wantInputs: 0,
		},
		{
			name:       "click-only returns wrapped ErrEventUnhandled",
			make:       func(rec *clickInputSetRecorder) any { return clickOnlyComboHandler{rec: rec} },
			clickRet:   wrappedUnhandled,
			wantErr:    ErrEventUnhandled,
			wantClicks: 1,
			wantInputs: 0,
		},
		{
			name:       "input-only is not used for click",
			make:       func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			inputRet:   nil,
			wantErr:    ErrEventUnhandled,
			wantClicks: 0,
			wantInputs: 0,
		},
		{
			name:       "dual returns nil from click",
			make:       func(rec *clickInputSetRecorder) any { return dualClickInputComboHandler{rec: rec} },
			wantErr:    nil,
			wantClicks: 1,
			wantInputs: 0,
		},
		{
			name:       "dual does not fall back from click to input",
			make:       func(rec *clickInputSetRecorder) any { return dualClickInputComboHandler{rec: rec} },
			clickRet:   ErrEventUnhandled,
			inputRet:   nil,
			wantErr:    ErrEventUnhandled,
			wantClicks: 1,
			wantInputs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &clickInputSetRecorder{
				clickRet: tt.clickRet,
				inputRet: tt.inputRet,
			}
			handler := tt.make(rec)

			err := CallEventHandlers(handler, elem, what.Click, "1 2 5 name")
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if rec.clickCalls != tt.wantClicks {
				t.Fatalf("click calls = %d, want %d", rec.clickCalls, tt.wantClicks)
			}
			if rec.inputCalls != tt.wantInputs {
				t.Fatalf("input calls = %d, want %d", rec.inputCalls, tt.wantInputs)
			}
		})
	}
}

func Test_CallEventHandlers_PointerDispatchCombinations(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	elem := rq.NewElement(testDivWidget{inner: "x"})
	wrappedUnhandled := fmt.Errorf("wrapped: %w", ErrEventUnhandled)

	tests := []struct {
		name         string
		make         func(*clickInputSetRecorder) any
		pointerRet   error
		inputRet     error
		wantErr      error
		wantPointers int
		wantInputs   int
	}{
		{
			name:         "pointer-only returns nil",
			make:         func(rec *clickInputSetRecorder) any { return pointerOnlyComboHandler{rec: rec} },
			wantErr:      nil,
			wantPointers: 1,
			wantInputs:   0,
		},
		{
			name:         "pointer-only returns ErrEventUnhandled",
			make:         func(rec *clickInputSetRecorder) any { return pointerOnlyComboHandler{rec: rec} },
			pointerRet:   ErrEventUnhandled,
			wantErr:      ErrEventUnhandled,
			wantPointers: 1,
			wantInputs:   0,
		},
		{
			name:         "pointer-only returns wrapped ErrEventUnhandled",
			make:         func(rec *clickInputSetRecorder) any { return pointerOnlyComboHandler{rec: rec} },
			pointerRet:   wrappedUnhandled,
			wantErr:      ErrEventUnhandled,
			wantPointers: 1,
			wantInputs:   0,
		},
		{
			name:         "input-only is not used for pointer",
			make:         func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			inputRet:     nil,
			wantErr:      ErrEventUnhandled,
			wantPointers: 0,
			wantInputs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &clickInputSetRecorder{
				pointerRet: tt.pointerRet,
				inputRet:   tt.inputRet,
			}
			handler := tt.make(rec)

			err := CallEventHandlers(handler, elem, what.Pointer, "move 1.5 2.25 5 -1 1 name")
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if rec.pointerCalls != tt.wantPointers {
				t.Fatalf("pointer calls = %d, want %d", rec.pointerCalls, tt.wantPointers)
			}
			if rec.inputCalls != tt.wantInputs {
				t.Fatalf("input calls = %d, want %d", rec.inputCalls, tt.wantInputs)
			}
		})
	}
}

func Test_CallEventHandlers_InputAndSetDispatchCombinations(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	elem := rq.NewElement(testDivWidget{inner: "x"})
	wrappedUnhandled := fmt.Errorf("wrapped: %w", ErrEventUnhandled)

	tests := []struct {
		name       string
		wht        what.What
		make       func(*clickInputSetRecorder) any
		inputRet   error
		wantErr    error
		wantInputs int
		wantClicks int
		inputVal   string
	}{
		{
			name:       "input-only handles Input",
			wht:        what.Input,
			make:       func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			wantErr:    nil,
			wantInputs: 1,
			inputVal:   "typed",
		},
		{
			name:       "input-only handles Hook",
			wht:        what.Hook,
			make:       func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			wantErr:    nil,
			wantInputs: 1,
			inputVal:   "sync",
		},
		{
			name:       "input-only returns ErrEventUnhandled",
			wht:        what.Input,
			make:       func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			inputRet:   ErrEventUnhandled,
			wantErr:    ErrEventUnhandled,
			wantInputs: 1,
			inputVal:   "typed",
		},
		{
			name:       "input-only returns wrapped ErrEventUnhandled",
			wht:        what.Input,
			make:       func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			inputRet:   wrappedUnhandled,
			wantErr:    ErrEventUnhandled,
			wantInputs: 1,
			inputVal:   "typed",
		},
		{
			name:       "input-only handles Set",
			wht:        what.Set,
			make:       func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			wantErr:    nil,
			wantInputs: 1,
			inputVal:   `x=1`,
		},
		{
			name:       "input-only Set returns ErrEventUnhandled",
			wht:        what.Set,
			make:       func(rec *clickInputSetRecorder) any { return inputOnlyComboHandler{rec: rec} },
			inputRet:   ErrEventUnhandled,
			wantErr:    ErrEventUnhandled,
			wantInputs: 1,
			inputVal:   `x=1`,
		},
		{
			name:       "click-only not used for Set",
			wht:        what.Set,
			make:       func(rec *clickInputSetRecorder) any { return clickOnlyComboHandler{rec: rec} },
			wantErr:    ErrEventUnhandled,
			wantClicks: 0,
			inputVal:   `x=1`,
		},
		{
			name:       "click-only not used for Input",
			wht:        what.Input,
			make:       func(rec *clickInputSetRecorder) any { return clickOnlyComboHandler{rec: rec} },
			wantErr:    ErrEventUnhandled,
			wantClicks: 0,
			inputVal:   "typed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &clickInputSetRecorder{
				inputRet: tt.inputRet,
			}
			handler := tt.make(rec)

			err := CallEventHandlers(handler, elem, tt.wht, tt.inputVal)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if rec.inputCalls != tt.wantInputs {
				t.Fatalf("input calls = %d, want %d", rec.inputCalls, tt.wantInputs)
			}
			if rec.clickCalls != tt.wantClicks {
				t.Fatalf("click calls = %d, want %d", rec.clickCalls, tt.wantClicks)
			}
		})
	}
}

func Test_CallEventHandlers_ExtrasOverrideUI_Click(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	t.Run("extra handles before ui", func(t *testing.T) {
		elem := rq.NewElement(testDivWidget{inner: "x"})
		extra := &clickInputSetRecorder{}
		ui := &clickInputSetRecorder{}
		elem.AddHandlers(clickOnlyComboHandler{rec: extra})

		err := CallEventHandlers(clickOnlyComboHandler{rec: ui}, elem, what.Click, "1 2 5 name")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if extra.clickCalls != 1 {
			t.Fatalf("extra click calls = %d, want 1", extra.clickCalls)
		}
		if ui.clickCalls != 0 {
			t.Fatalf("ui click calls = %d, want 0", ui.clickCalls)
		}
	})

	t.Run("ui fallback after extra unhandled", func(t *testing.T) {
		elem := rq.NewElement(testDivWidget{inner: "x"})
		extra := &clickInputSetRecorder{clickRet: ErrEventUnhandled}
		ui := &clickInputSetRecorder{}
		elem.AddHandlers(clickOnlyComboHandler{rec: extra})

		err := CallEventHandlers(clickOnlyComboHandler{rec: ui}, elem, what.Click, "1 2 5 name")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if extra.clickCalls != 1 {
			t.Fatalf("extra click calls = %d, want 1", extra.clickCalls)
		}
		if ui.clickCalls != 1 {
			t.Fatalf("ui click calls = %d, want 1", ui.clickCalls)
		}
	})
}

func Test_CallEventHandlers_ExtrasOverrideUI_Pointer(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	t.Run("extra handles before ui", func(t *testing.T) {
		elem := rq.NewElement(testDivWidget{inner: "x"})
		extra := &clickInputSetRecorder{}
		ui := &clickInputSetRecorder{}
		elem.AddHandlers(pointerOnlyComboHandler{rec: extra})

		err := CallEventHandlers(pointerOnlyComboHandler{rec: ui}, elem, what.Pointer, "move 1.5 2.25 5 -1 1 name")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if extra.pointerCalls != 1 {
			t.Fatalf("extra pointer calls = %d, want 1", extra.pointerCalls)
		}
		if ui.pointerCalls != 0 {
			t.Fatalf("ui pointer calls = %d, want 0", ui.pointerCalls)
		}
	})

	t.Run("ui fallback after extra unhandled", func(t *testing.T) {
		elem := rq.NewElement(testDivWidget{inner: "x"})
		extra := &clickInputSetRecorder{pointerRet: ErrEventUnhandled}
		ui := &clickInputSetRecorder{}
		elem.AddHandlers(pointerOnlyComboHandler{rec: extra})

		err := CallEventHandlers(pointerOnlyComboHandler{rec: ui}, elem, what.Pointer, "move 1.5 2.25 5 -1 1 name")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if extra.pointerCalls != 1 {
			t.Fatalf("extra pointer calls = %d, want 1", extra.pointerCalls)
		}
		if ui.pointerCalls != 1 {
			t.Fatalf("ui pointer calls = %d, want 1", ui.pointerCalls)
		}
	})
}

func Test_CallEventHandlers_ExtrasOverrideUI_InputAndSet(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tests := []struct {
		name string
		wht  what.What
		val  string
	}{
		{name: "input", wht: what.Input, val: "typed"},
		{name: "set", wht: what.Set, val: `x=1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("extra handles before ui", func(t *testing.T) {
				elem := rq.NewElement(testDivWidget{inner: "x"})
				extra := &clickInputSetRecorder{}
				ui := &clickInputSetRecorder{}
				elem.AddHandlers(inputOnlyComboHandler{rec: extra})

				err := CallEventHandlers(inputOnlyComboHandler{rec: ui}, elem, tt.wht, tt.val)
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				if extra.inputCalls != 1 {
					t.Fatalf("extra input calls = %d, want 1", extra.inputCalls)
				}
				if ui.inputCalls != 0 {
					t.Fatalf("ui input calls = %d, want 0", ui.inputCalls)
				}
			})

			t.Run("ui fallback after extra unhandled", func(t *testing.T) {
				elem := rq.NewElement(testDivWidget{inner: "x"})
				extra := &clickInputSetRecorder{inputRet: ErrEventUnhandled}
				ui := &clickInputSetRecorder{}
				elem.AddHandlers(inputOnlyComboHandler{rec: extra})

				err := CallEventHandlers(inputOnlyComboHandler{rec: ui}, elem, tt.wht, tt.val)
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				if extra.inputCalls != 1 {
					t.Fatalf("extra input calls = %d, want 1", extra.inputCalls)
				}
				if ui.inputCalls != 1 {
					t.Fatalf("ui input calls = %d, want 1", ui.inputCalls)
				}
			})
		})
	}
}

func Test_CallEventHandlers_ExtraHandlersAreLIFO_Click(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	first := &clickInputSetRecorder{}
	last := &clickInputSetRecorder{}
	ui := &clickInputSetRecorder{}

	elem.AddHandlers(
		clickOnlyComboHandler{rec: first},
		clickOnlyComboHandler{rec: last},
	)

	err := CallEventHandlers(clickOnlyComboHandler{rec: ui}, elem, what.Click, "1 2 5 name")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if last.clickCalls != 1 {
		t.Fatalf("last click calls = %d, want 1", last.clickCalls)
	}
	if first.clickCalls != 0 {
		t.Fatalf("first click calls = %d, want 0", first.clickCalls)
	}
	if ui.clickCalls != 0 {
		t.Fatalf("ui click calls = %d, want 0", ui.clickCalls)
	}

	last.clickRet = ErrEventUnhandled
	err = CallEventHandlers(clickOnlyComboHandler{rec: ui}, elem, what.Click, "1 2 5 name")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if last.clickCalls != 2 {
		t.Fatalf("last click calls = %d, want 2", last.clickCalls)
	}
	if first.clickCalls != 1 {
		t.Fatalf("first click calls = %d, want 1", first.clickCalls)
	}
	if ui.clickCalls != 0 {
		t.Fatalf("ui click calls = %d, want 0", ui.clickCalls)
	}
}

func Test_CallEventHandlers_ExtraHandlersAreLIFO_Pointer(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	first := &clickInputSetRecorder{}
	last := &clickInputSetRecorder{}
	ui := &clickInputSetRecorder{}

	elem.AddHandlers(
		pointerOnlyComboHandler{rec: first},
		pointerOnlyComboHandler{rec: last},
	)

	err := CallEventHandlers(pointerOnlyComboHandler{rec: ui}, elem, what.Pointer, "move 1.5 2.25 5 -1 1 name")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if last.pointerCalls != 1 {
		t.Fatalf("last pointer calls = %d, want 1", last.pointerCalls)
	}
	if first.pointerCalls != 0 {
		t.Fatalf("first pointer calls = %d, want 0", first.pointerCalls)
	}
	if ui.pointerCalls != 0 {
		t.Fatalf("ui pointer calls = %d, want 0", ui.pointerCalls)
	}

	last.pointerRet = ErrEventUnhandled
	err = CallEventHandlers(pointerOnlyComboHandler{rec: ui}, elem, what.Pointer, "move 1.5 2.25 5 -1 1 name")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if last.pointerCalls != 2 {
		t.Fatalf("last pointer calls = %d, want 2", last.pointerCalls)
	}
	if first.pointerCalls != 1 {
		t.Fatalf("first pointer calls = %d, want 1", first.pointerCalls)
	}
	if ui.pointerCalls != 0 {
		t.Fatalf("ui pointer calls = %d, want 0", ui.pointerCalls)
	}
}

func Test_CallEventHandlers_ExtraHandlersAreLIFO_InputAndSet(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tests := []struct {
		name string
		wht  what.What
		val  string
	}{
		{name: "input", wht: what.Input, val: "typed"},
		{name: "set", wht: what.Set, val: `x=1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem := rq.NewElement(testDivWidget{inner: "x"})
			first := &clickInputSetRecorder{}
			last := &clickInputSetRecorder{}
			ui := &clickInputSetRecorder{}

			elem.AddHandlers(
				inputOnlyComboHandler{rec: first},
				inputOnlyComboHandler{rec: last},
			)

			err := CallEventHandlers(inputOnlyComboHandler{rec: ui}, elem, tt.wht, tt.val)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if last.inputCalls != 1 {
				t.Fatalf("last input calls = %d, want 1", last.inputCalls)
			}
			if first.inputCalls != 0 {
				t.Fatalf("first input calls = %d, want 0", first.inputCalls)
			}
			if ui.inputCalls != 0 {
				t.Fatalf("ui input calls = %d, want 0", ui.inputCalls)
			}

			last.inputRet = ErrEventUnhandled
			err = CallEventHandlers(inputOnlyComboHandler{rec: ui}, elem, tt.wht, tt.val)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if last.inputCalls != 2 {
				t.Fatalf("last input calls = %d, want 2", last.inputCalls)
			}
			if first.inputCalls != 1 {
				t.Fatalf("first input calls = %d, want 1", first.inputCalls)
			}
			if ui.inputCalls != 0 {
				t.Fatalf("ui input calls = %d, want 0", ui.inputCalls)
			}
		})
	}
}

func Test_CallEventHandlers_PanicError(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	wantErr := fmt.Errorf("boom")
	err := CallEventHandlers(testPanicInputHandler{panicVal: wantErr}, elem, what.Input, "")
	if !errors.Is(err, ErrEventHandlerPanic) {
		t.Errorf("got %v, want ErrEventHandlerPanic", err)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Unwrap: got %v, want %v", errors.Unwrap(err), wantErr)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("Error() = %q, want it to contain %q", err.Error(), "boom")
	}
}

func Test_CallEventHandlers_PanicString(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	err := CallEventHandlers(testPanicInputHandler{panicVal: "oops"}, elem, what.Input, "")
	if !errors.Is(err, ErrEventHandlerPanic) {
		t.Errorf("got %v, want ErrEventHandlerPanic", err)
	}
	if errors.Unwrap(err) != nil {
		t.Errorf("Unwrap: got %v, want nil", errors.Unwrap(err))
	}
	if !strings.Contains(err.Error(), "oops") {
		t.Errorf("Error() = %q, want it to contain %q", err.Error(), "oops")
	}
}

func Test_CallEventHandlers_ClickOnlyHandlerViaApplyGetter(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	clickCounter := &testClickCounter{wantName: "name"}
	if _, _, err := elem.ApplyGetter(clickCounter); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}

	err := CallEventHandlers(elem.Ui(), elem, what.Click, "1 2 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click handler to be called once, got %d", clickCounter.n)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.Click, "1 2 0 wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click count to stay 1 for wrong name, got %d", clickCounter.n)
	}
}

func Test_CallEventHandlers_ClickOnlyHandlerViaApplyParams(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	clickCounter := &testClickCounter{wantName: "name"}
	elem.ApplyParams([]any{clickCounter})

	err := CallEventHandlers(elem.Ui(), elem, what.Click, "1 2 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click handler to be called once, got %d", clickCounter.n)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.Click, "1 2 0 wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click count to stay 1 for wrong name, got %d", clickCounter.n)
	}
}

func Test_CallEventHandlers_ContextMenuOnlyHandlerViaApplyGetter(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	counter := &testContextMenuCounter{wantName: "name"}
	if _, _, err := elem.ApplyGetter(counter); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}

	err := CallEventHandlers(elem.Ui(), elem, what.ContextMenu, "10 20 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected context menu handler to be called once, got %d", counter.n)
	}
	if counter.lastValue != (Click{Name: "name", X: 10, Y: 20, Shift: true, Alt: true}) {
		t.Fatalf("unexpected click payload %+v", counter.lastValue)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.ContextMenu, "10 20 0 wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected count to stay 1 for wrong name, got %d", counter.n)
	}
}

func Test_CallEventHandlers_ContextMenuOnlyHandlerViaApplyParams(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	counter := &testContextMenuCounter{wantName: "name"}
	elem.ApplyParams([]any{counter})

	err := CallEventHandlers(elem.Ui(), elem, what.ContextMenu, "10 20 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected context menu handler to be called once, got %d", counter.n)
	}
	if counter.lastValue != (Click{Name: "name", X: 10, Y: 20, Shift: true, Alt: true}) {
		t.Fatalf("unexpected click payload %+v", counter.lastValue)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.ContextMenu, "10 20 0 wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected count to stay 1 for wrong name, got %d", counter.n)
	}
}

func Test_CallEventHandlers_PointerOnlyHandlerViaApplyGetter(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	counter := &testPointerCounter{wantName: "name"}
	if _, _, err := elem.ApplyGetter(counter); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}

	err := CallEventHandlers(elem.Ui(), elem, what.Pointer, "move 10.5 20.25 5 -1 1 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected pointer handler to be called once, got %d", counter.n)
	}
	if counter.lastValue != (Pointer{Name: "name", X: 10.5, Y: 20.25, Kind: PointerMove, Button: -1, Buttons: PointerButtonPrimary, Shift: true, Alt: true}) {
		t.Fatalf("unexpected pointer payload %+v", counter.lastValue)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.Pointer, "move 10 20 0 -1 1 wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected count to stay 1 for wrong name, got %d", counter.n)
	}
}

func Test_CallEventHandlers_PointerOnlyHandlerViaApplyParams(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	counter := &testPointerCounter{wantName: "name"}
	elem.ApplyParams([]any{counter})

	err := CallEventHandlers(elem.Ui(), elem, what.Pointer, "move 10.5 20.25 5 -1 1 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected pointer handler to be called once, got %d", counter.n)
	}
	if counter.lastValue != (Pointer{Name: "name", X: 10.5, Y: 20.25, Kind: PointerMove, Button: -1, Buttons: PointerButtonPrimary, Shift: true, Alt: true}) {
		t.Fatalf("unexpected pointer payload %+v", counter.lastValue)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.Pointer, "move 10 20 0 -1 1 wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected count to stay 1 for wrong name, got %d", counter.n)
	}
}

func Test_JawsInput_ExtraHandler(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)

	ih := &testJawsInputHandler{msgCh: msgCh}

	var sb strings.Builder
	elem := rq.NewElement(testDivWidget{inner: "tjIH"})
	th.NoErr(elem.JawsRender(&sb, []any{ih}))
	th.Equal(sb.String(), "<div id=\"Jid.1\">tjIH</div>")

	rq.InCh <- wire.WsMsg{Data: "typed", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		th.Equal(s, `JawsInput: "typed"`)
	}
}
