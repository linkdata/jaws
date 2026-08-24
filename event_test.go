package jaws

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestParseClickData(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Click
		wantRem string
		wantOK  bool
	}{
		{
			name:   "without modifiers",
			in:     "10 20 0 save",
			want:   Click{Name: "save", X: 10, Y: 20},
			wantOK: true,
		},
		{
			name:   "fractional coordinates",
			in:     "10.25 20.5 0 save",
			want:   Click{Name: "save", X: 10.25, Y: 20.5},
			wantOK: true,
		},
		{
			name:    "with modifiers and route",
			in:      "10 20 5 save\tJid.1\tJid.2",
			want:    Click{Name: "save", X: 10, Y: 20, Shift: true, Alt: true},
			wantRem: "Jid.1\tJid.2",
			wantOK:  true,
		},
		{
			name:    "route without modifiers",
			in:      "10 20 0 save\tJid.1",
			want:    Click{Name: "save", X: 10, Y: 20},
			wantRem: "Jid.1",
			wantOK:  true,
		},
		{
			name:    "one modifier and route",
			in:      "10 20 2 save\tJid.1",
			want:    Click{Name: "save", X: 10, Y: 20, Control: true},
			wantRem: "Jid.1",
			wantOK:  true,
		},
		{
			name:   "name with spaces",
			in:     "10 20 2 save button",
			want:   Click{Name: "save button", X: 10, Y: 20, Control: true},
			wantOK: true,
		},
		{
			name:   "name with many tokens collapses whitespace",
			in:     "1 2 0    a   b     c d",
			want:   Click{Name: "a b c d", X: 1, Y: 2},
			wantOK: true,
		},
		{
			name:   "invalid x",
			in:     "bad 20 0 save",
			wantOK: false,
		},
		{
			name:   "invalid y",
			in:     "10 bad 0 save",
			wantOK: false,
		},
		{
			name:   "invalid keystate",
			in:     "10 20 bad save",
			wantOK: false,
		},
		{
			name:   "unknown keystate bit",
			in:     "10 20 8 save",
			wantOK: false,
		},
		{
			name:   "negative keystate",
			in:     "10 20 -1 save",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rem, ok := parseClickData(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Fatalf("click = %+v, want %+v", got, tt.want)
			}
			if rem != tt.wantRem {
				t.Fatalf("after = %q, want %q", rem, tt.wantRem)
			}
		})
	}
}

func TestParseClickDataAcceptsNonFinite(t *testing.T) {
	// runAtof no longer rejects non-finite coordinates; the event dispatch terminates
	// the Request instead (see TestCallEventHandlerTerminatesOnNonFiniteClick).
	tests := []struct {
		name  string
		in    string
		check func(Click) bool
	}{
		{"nan x", "NaN 20 0 save", func(c Click) bool { return math.IsNaN(c.X) && c.Y == 20 }},
		{"infinite x", "+Inf 20 0 save", func(c Click) bool { return math.IsInf(c.X, 1) && c.Y == 20 }},
		{"nan y", "10 NaN 0 save", func(c Click) bool { return c.X == 10 && math.IsNaN(c.Y) }},
		{"infinite y", "10 -Inf 0 save", func(c Click) bool { return c.X == 10 && math.IsInf(c.Y, -1) }},
		{"overflow x", "1e999 20 0 save", func(c Click) bool { return math.IsInf(c.X, 1) && c.Y == 20 }},
		{"overflow y", "10 -1e999 0 save", func(c Click) bool { return c.X == 10 && math.IsInf(c.Y, -1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk, _, ok := parseClickData(tt.in)
			if !ok {
				t.Fatalf("ok = false, want true")
			}
			if !tt.check(clk) {
				t.Fatalf("unexpected click %+v", clk)
			}
		})
	}
}

func TestFinite(t *testing.T) {
	tests := []struct {
		f    float64
		want bool
	}{
		{0, true},
		{-1.5, true},
		{math.MaxFloat64, true},
		{math.NaN(), false},
		{math.Inf(1), false},
		{math.Inf(-1), false},
	}
	for _, tt := range tests {
		if got := finite(tt.f); got != tt.want {
			t.Errorf("finite(%v) = %v, want %v", tt.f, got, tt.want)
		}
	}
}

func TestClickString(t *testing.T) {
	for _, tt := range []struct {
		name string
		clk  Click
		want string
	}{
		{
			name: "canonical",
			clk:  Click{Name: "x", X: 1.25, Y: 2.5, Shift: true, Control: true, Alt: true},
			want: "1.25 2.5 7 x",
		},
		{
			name: "name whitespace",
			clk:  Click{Name: " \tsave\n all\r ", X: 1.25, Y: 2.5},
			want: "1.25 2.5 0 save all",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded := tt.clk.String()
			if got := encoded; got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
			got, after, ok := parseClickData(encoded)
			if !ok {
				t.Fatalf("parseClickData(String()) failed for %q", encoded)
			}
			if after != "" {
				t.Fatalf("trailing data = %q, want none", after)
			}
			if want := strings.Join(strings.Fields(tt.clk.Name), " "); got.Name != want {
				t.Fatalf("parsed name = %q, want %q", got.Name, want)
			}
		})
	}
}

func Fuzz_parseClickData(f *testing.F) {
	f.Add("1 2 0 name")
	f.Add("1 2 5 name")
	f.Add("1 2 5 name\tJid.1\tJid.2")
	f.Add("1 2 0 name\tJid.1")
	f.Add("bad 2 0 name")
	f.Fuzz(func(t *testing.T, in string) {
		clk, after, ok := parseClickData(in)
		if !ok {
			return
		}
		// parseClickData accepts non-finite coordinates (the event dispatch terminates
		// the Request on them); Click.String round-trips only finite values, since
		// NaN != NaN defeats the equality check below.
		if !finite(clk.X) || !finite(clk.Y) {
			return
		}

		encoded := clk.String()
		clk2, after2, ok := parseClickData(encoded)
		if !ok {
			t.Fatalf("parseClickData(Click.String()) failed: click=%+v encoded=%q", clk, encoded)
		}
		if clk2 != clk || after2 != "" {
			t.Fatalf("parseClickData(Click.String()) mismatch: click=%+v got=%+v after=%q", clk, clk2, after2)
		}

		roundtripInput := encoded
		if after != "" {
			roundtripInput += "\t" + after
		}
		clk3, after3, ok := parseClickData(roundtripInput)
		if !ok {
			t.Fatalf("parseClickData(roundtrip input) failed: input=%q", roundtripInput)
		}
		if clk3 != clk || after3 != after {
			t.Fatalf("roundtrip mismatch: click=%+v/%+v after=%q/%q", clk, clk3, after, after3)
		}
	})
}

func BenchmarkParseClickData(b *testing.B) {
	// Name accumulation must remain linear in the frame size, and the single-token
	// click (by far the most common) must stay allocation-free. Cover one, two, and
	// many name tokens; the long name is bounded in production by the WebSocket read
	// limit.
	cases := []struct {
		name  string
		value string
	}{
		{"OneToken", "10 20 0 save"},
		{"TwoTokens", "10 20 0 save button"},
		{"LongName", "0 0 0 " + strings.Repeat("a ", 8000)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, ok := parseClickData(tc.value); !ok {
					b.Fatal("expected ok")
				}
			}
		})
	}
}

func Fuzz_clickStringRoundTrip(f *testing.F) {
	f.Add("name", int32(1), int32(2), true, false, true)
	f.Add("button", int32(-1), int32(999), false, false, false)
	f.Add("save\tall", int32(0), int32(0), false, false, false)
	f.Fuzz(func(t *testing.T, name string, x int32, y int32, shift, control, alt bool) {
		clk := Click{
			Name:    name,
			X:       float64(x),
			Y:       float64(y),
			Shift:   shift,
			Control: control,
			Alt:     alt,
		}
		got, after, ok := parseClickData(clk.String())
		if !ok {
			t.Fatalf("parseClickData(String()) failed: click=%+v", clk)
		}
		if after != "" {
			t.Fatalf("expected no trailing data, got %q", after)
		}
		want := clk
		want.Name = strings.Join(strings.Fields(name), " ")
		if got != want {
			t.Fatalf("roundtrip mismatch: want=%+v got=%+v", want, got)
		}
	})
}

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

func (t *testJawsEvent) JawsGetTag() (tagValue any) {
	return t.tagValue
}

func (t *testJawsEvent) JawsRender(elem *Element, w io.Writer, params []any) (err error) {
	tagValue := elem.ApplyGetter(t)
	_, _ = fmt.Fprint(w, params)
	t.msgCh <- fmt.Sprintf("JawsRender(%d)%#v", elem.jid, tagValue)
	return
}

func (t *testJawsEvent) JawsUpdate(elem *Element) {
	t.msgCh <- fmt.Sprintf("JawsUpdate(%d)", elem.jid)
}

var (
	_ ClickHandler  = (*testJawsEvent)(nil)
	_ InputHandler  = (*testJawsEvent)(nil)
	_ tag.TagGetter = (*testJawsEvent)(nil)
	_ UI            = (*testJawsEvent)(nil)
)

func Test_JawsInput_InvokesJawsInputForDualHandler(t *testing.T) {
	th := newTestHelper(t)
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

type clickInputSetRecorder struct {
	clickRet   error
	inputRet   error
	clickCalls int
	inputCalls int
}

type clickOnlyComboHandler struct{ rec *clickInputSetRecorder }

func (h clickOnlyComboHandler) JawsClick(elem *Element, click Click) error {
	h.rec.clickCalls++
	return h.rec.clickRet
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

func TestRequest_CallAllEventHandlersRequiresFrozenElement(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	t.Run("direct", func(t *testing.T) {
		rec := &clickInputSetRecorder{}
		elem := rq.NewElement(testDivWidget{inner: "x"})
		elem.AddHandlers(inputOnlyComboHandler{rec: rec})

		if err := rq.callAllEventHandlers(elem.Jid(), what.Input, "before"); err != nil {
			t.Fatal(err)
		}
		if rec.inputCalls != 0 {
			t.Fatalf("input calls before Freeze = %d, want 0", rec.inputCalls)
		}

		elem.Freeze()
		if err := rq.callAllEventHandlers(elem.Jid(), what.Input, "after"); err != nil {
			t.Fatal(err)
		}
		if rec.inputCalls != 1 {
			t.Fatalf("input calls after Freeze = %d, want 1", rec.inputCalls)
		}
	})

	t.Run("bubbled", func(t *testing.T) {
		rec := &clickInputSetRecorder{}
		elem := rq.NewElement(testDivWidget{inner: "x"})
		elem.AddHandlers(clickOnlyComboHandler{rec: rec})
		value := "1 2 0 name\t" + elem.Jid().String() + "\t"

		if err := rq.callAllEventHandlers(0, what.Click, value); err != nil {
			t.Fatal(err)
		}
		if rec.clickCalls != 0 {
			t.Fatalf("click calls before Freeze = %d, want 0", rec.clickCalls)
		}

		elem.Freeze()
		if err := rq.callAllEventHandlers(0, what.Click, value); err != nil {
			t.Fatal(err)
		}
		if rec.clickCalls != 1 {
			t.Fatalf("click calls after Freeze = %d, want 1", rec.clickCalls)
		}
	})
}

func TestCallEventHandlerTerminatesOnNonFiniteClick(t *testing.T) {
	tests := []struct {
		name  string
		wht   what.What
		click string
	}{
		{"click nan x", what.Click, "NaN 2 0 name"},
		{"click +inf y", what.Click, "1 +Inf 0 name"},
		{"click -inf x", what.Click, "-Inf 2 0 name"},
		{"click overflow x", what.Click, "1e999 2 0 name"},
		{"contextmenu nan y", what.ContextMenu, "1 NaN 0 name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq := newTestRequest(t)
			defer rq.Close()
			rec := &clickInputSetRecorder{}
			elem := rq.NewElement(testDivWidget{inner: "x"})
			elem.AddHandlers(clickOnlyComboHandler{rec: rec})
			elem.Freeze()

			value := tt.click + "\t" + elem.Jid().String() + "\t"
			// The dispatch reports the event handled (nil) after terminating, so the
			// dying connection is not also sent an alert.
			if err := rq.callAllEventHandlers(0, tt.wht, value); err != nil {
				t.Fatalf("callAllEventHandlers err = %v, want nil", err)
			}
			if cause := context.Cause(rq.Context()); !errors.Is(cause, ErrValueNotFinite) {
				t.Fatalf("cause = %v, want wrapping ErrValueNotFinite", cause)
			}
			if rec.clickCalls != 0 {
				t.Fatalf("click handler fired %d times, want 0", rec.clickCalls)
			}
		})
	}
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

// Test_CallEventHandlers_PanicNonComparable guards that a handler panicking with a
// non-comparable value (here a map) flows through errors.Is against the exported
// sentinel without panicking. ErrEventHandlerPanic carries a nil Value, so comparing
// the wrapped error against it never reaches a map-to-map comparison; this locks in
// that the reachable matching path stays panic-safe.
func Test_CallEventHandlers_PanicNonComparable(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	err := CallEventHandlers(testPanicInputHandler{panicVal: map[int]int{1: 1}}, elem, what.Input, "")
	if !errors.Is(err, ErrEventHandlerPanic) {
		t.Errorf("got %v, want ErrEventHandlerPanic", err)
	}
	// The non-comparable value is not an error, so Unwrap yields nil rather than
	// exposing the map.
	if errors.Unwrap(err) != nil {
		t.Errorf("Unwrap: got %v, want nil", errors.Unwrap(err))
	}
}

func Test_CallEventHandlers_ClickOnlyHandlerViaApplyGetter(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	clickCounter := &testClickCounter{wantName: "name"}
	elem.ApplyGetter(clickCounter)

	err := CallEventHandlers(elem.UI(), elem, what.Click, "1 2 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click handler to be called once, got %d", clickCounter.n)
	}
	err = CallEventHandlers(elem.UI(), elem, what.Click, "1 2 0 wrong")
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

	err := CallEventHandlers(elem.UI(), elem, what.Click, "1 2 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click handler to be called once, got %d", clickCounter.n)
	}
	err = CallEventHandlers(elem.UI(), elem, what.Click, "1 2 0 wrong")
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
	elem.ApplyGetter(counter)

	err := CallEventHandlers(elem.UI(), elem, what.ContextMenu, "10 20 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected context menu handler to be called once, got %d", counter.n)
	}
	if counter.lastValue != (Click{Name: "name", X: 10, Y: 20, Shift: true, Alt: true}) {
		t.Fatalf("unexpected click payload %+v", counter.lastValue)
	}
	err = CallEventHandlers(elem.UI(), elem, what.ContextMenu, "10 20 0 wrong")
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

	err := CallEventHandlers(elem.UI(), elem, what.ContextMenu, "10 20 5 name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected context menu handler to be called once, got %d", counter.n)
	}
	if counter.lastValue != (Click{Name: "name", X: 10, Y: 20, Shift: true, Alt: true}) {
		t.Fatalf("unexpected click payload %+v", counter.lastValue)
	}
	err = CallEventHandlers(elem.UI(), elem, what.ContextMenu, "10 20 0 wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("expected count to stay 1 for wrong name, got %d", counter.n)
	}
}

func Test_JawsInput_ExtraHandler(t *testing.T) {
	th := newTestHelper(t)
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

type testJawsClick struct {
	clickCh chan string
	*testSetter[string]
}

func (tjc *testJawsClick) JawsClick(elem *Element, click Click) (err error) {
	if err = tjc.Err(); err == nil {
		tjc.clickCh <- click.Name
	}
	return
}

var _ ClickHandler = (*testJawsClick)(nil)

type testJawsContextMenu struct {
	clickCh chan Click
	*testSetter[Click]
}

func (tjc *testJawsContextMenu) JawsContextMenu(elem *Element, click Click) (err error) {
	if err = tjc.Err(); err == nil {
		tjc.clickCh <- click
	}
	return
}

var _ ContextMenuHandler = (*testJawsContextMenu)(nil)

type testJawsInitialHTMLAttr struct{}

func (testJawsInitialHTMLAttr) JawsInitialHTMLAttr(elem *Element) template.HTMLAttr {
	return `data-test="1"`
}

var _ InitialHTMLAttrHandler = testJawsInitialHTMLAttr{}

func Test_clickHandlerWrapper_Dispatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		tjc := &testJawsClick{
			clickCh:    make(chan string),
			testSetter: newTestSetter(""),
		}

		want := `<div id="Jid.1">inner</div>`
		if err := rq.UI(testDivWidget{inner: template.HTML("inner")}, tjc); err != nil {
			t.Fatal(err)
		}
		if got := rq.BodyString(); got != want {
			t.Errorf("Request.UI(NewDiv()) = %q, want %q", got, want)
		}

		// An Input message to a div (which has no input handler) must produce no
		// output. synctest.Wait blocks until the process loop has fully handled the
		// message, so the negative assertion is not vacuous: a bare default: select
		// would short-circuit before the async process goroutine could react.
		rq.InCh <- wire.WsMsg{Data: "text", Jid: 1, What: what.Input}
		synctest.Wait()
		select {
		case s := <-rq.OutCh:
			t.Errorf("unexpected output for Input: %q", s.Format())
		default:
		}

		// A malformed click ("adam", missing coordinates) must be ignored before the
		// handler is invoked.
		rq.InCh <- wire.WsMsg{Data: "adam", Jid: 1, What: what.Click}
		synctest.Wait()
		select {
		case name := <-tjc.clickCh:
			t.Fatalf("malformed click should be ignored, got %q", name)
		default:
		}

		// A well-formed click dispatches to the handler.
		rq.InCh <- wire.WsMsg{Data: "1 2 0 adam", Jid: 1, What: what.Click}
		synctest.Wait()
		select {
		case name := <-tjc.clickCh:
			if name != "adam" {
				t.Error(name)
			}
		default:
			t.Fatal("expected click to dispatch to handler")
		}
	})
}

func Test_InitialHTMLAttrHandler_IgnoredByDispatch(t *testing.T) {
	if err := callEventHandler(testJawsInitialHTMLAttr{}, nil, what.Input, "ignored"); err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled, got %v", err)
	}
}
