package ui

import (
	"context"
	"errors"
	"html/template"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestInputTextWidgets(t *testing.T) {
	_, rq := newCoreRequest(t)
	ss := newTestSetter("foo")

	text := NewText(ss)
	elem, got := renderUI(t, rq, text)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="text" value="foo">$`, got)

	if err := text.JawsInput(elem, "bar"); err != nil {
		t.Fatal(err)
	}
	if ss.Get() != "bar" {
		t.Fatalf("want bar got %q", ss.Get())
	}
	if err := jaws.CallEventHandlers(text, elem, what.Click, "1 2 0 noop"); !errors.Is(err, jaws.ErrEventUnhandled) {
		t.Fatalf("want ErrEventUnhandled got %v", err)
	}
	ss.SetErr(errors.New("meh"))
	if err := text.JawsInput(elem, "omg"); err == nil || err.Error() != "meh" {
		t.Fatalf("want meh got %v", err)
	}
	ss.SetErr(nil)
	ss.Set("quux")
	text.JawsUpdate(elem)

	password := NewPassword(ss)
	_, got = renderUI(t, rq, password)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="password" value="quux">$`, got)

	textarea := NewTextarea(ss)
	textareaElem, got := renderUI(t, rq, textarea)
	mustMatch(t, `^<textarea id="Jid\.[0-9]+">\nquux</textarea>$`, got)
	textarea.JawsUpdate(textareaElem)
}

func TestInputBoolWidgets(t *testing.T) {
	_, rq := newCoreRequest(t)
	sb := newTestSetter(true)

	checkbox := NewCheckbox(sb)
	elem, got := renderUI(t, rq, checkbox)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="checkbox" checked>$`, got)
	if err := checkbox.JawsInput(elem, "false"); err != nil {
		t.Fatal(err)
	}
	if sb.Get() {
		t.Fatal("expected false")
	}
	if err := checkbox.JawsInput(elem, ""); err != nil {
		t.Fatal(err)
	}
	if sb.Get() {
		t.Fatal("expected false for empty input")
	}
	if err := checkbox.JawsInput(elem, "bad"); err == nil {
		t.Fatal("expected parse error")
	}
	sb.Set(true)
	checkbox.JawsUpdate(elem)

	radio := NewRadio(sb)
	_, got = renderUI(t, rq, radio)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="radio" checked>$`, got)
}

// TestInputBool_JawsUpdateEmitsCheckedState verifies that InputBool.JawsUpdate
// emits a SetValue carrying "true"/"false" on a genuine transition and nothing
// when the bound value is unchanged (exercising the u.Last.Swap dedup). jaws.js
// applies that literal text to the input's checked state.
func TestInputBool_JawsUpdateEmitsCheckedState(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	sb := newTestSetter(false)
	checkbox := NewCheckbox(sb)
	elem := tr.NewElement(checkbox)
	var buf strings.Builder
	if err := elem.JawsRender(&buf, nil); err != nil {
		t.Fatal(err)
	}

	waitValue := func() (string, bool) {
		deadline := time.After(300 * time.Millisecond)
		for {
			select {
			case msg := <-tr.OutCh:
				if msg.What == what.Value {
					return msg.Data, true
				}
			case <-deadline:
				return "", false
			}
		}
	}

	// false -> true emits "true".
	sb.Set(true)
	checkbox.JawsUpdate(elem)
	tr.InCh <- wire.WsMsg{} // wake the loop so the queued op flushes to OutCh
	if v, ok := waitValue(); !ok || v != "true" {
		t.Fatalf("expected SetValue %q on false->true, got ok=%v v=%q", "true", ok, v)
	}

	// true -> false emits "false".
	sb.Set(false)
	checkbox.JawsUpdate(elem)
	tr.InCh <- wire.WsMsg{}
	if v, ok := waitValue(); !ok || v != "false" {
		t.Fatalf("expected SetValue %q on true->false, got ok=%v v=%q", "false", ok, v)
	}

	// Unchanged value emits nothing (Last.Swap dedup).
	checkbox.JawsUpdate(elem)
	checkbox.JawsUpdate(elem)
	tr.InCh <- wire.WsMsg{}
	if v, ok := waitValue(); ok {
		t.Fatalf("unchanged bool value re-emitted SetValue %q", v)
	}
}

func TestInputFloatWidgets(t *testing.T) {
	_, rq := newCoreRequest(t)
	sf := newTestSetter(1.2)

	number := NewNumber(sf)
	elem, got := renderUI(t, rq, number)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="number" value="1.2">$`, got)
	if err := number.JawsInput(elem, "2.3"); err != nil {
		t.Fatal(err)
	}
	if sf.Get() != 2.3 {
		t.Fatalf("want 2.3 got %v", sf.Get())
	}
	if err := number.JawsInput(elem, ""); err != nil {
		t.Fatal(err)
	}
	if sf.Get() != 0 {
		t.Fatalf("want 0 got %v", sf.Get())
	}
	if err := number.JawsInput(elem, "bad"); err == nil {
		t.Fatal("expected parse error")
	}
	sf.Set(3.4)
	number.JawsUpdate(elem)

	rng := NewRange(sf)
	_, got = renderUI(t, rq, rng)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="range" value="3.4">$`, got)
}

// TestRangeRendersExplicitBounds guards the documented workaround for the browser
// range defaults: min, max and step passed as params render verbatim after the
// value, letting a caller widen a domain the default 0..100 would otherwise clamp.
func TestRangeRendersExplicitBounds(t *testing.T) {
	_, rq := newCoreRequest(t)
	sf := newTestSetter(150.0)
	_, got := renderUI(t, rq, NewRange(sf), `min="0"`, `max="200"`, `step="0.5"`)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="range" value="150" min="0" max="200" step="0.5">$`, got)
}

// TestInputFloat_ReconcilesConvertedValues verifies that number and range inputs
// receive the canonical server value when a numeric adapter truncates or rounds
// an input to the value that was already stored.
func TestInputFloat_ReconcilesConvertedValues(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       string
		makeSetter func() bind.Setter[float64]
	}{
		{
			name:  "int truncation",
			input: "1.9",
			want:  "1",
			makeSetter: func() bind.Setter[float64] {
				var mu deadlock.Mutex
				value := 1
				return bind.MakeSetterFloat64(bind.New(&mu, &value))
			},
		},
		{
			name:  "float32 rounding",
			input: "0.1",
			want:  "0.10000000149011612",
			makeSetter: func() bind.Setter[float64] {
				var mu deadlock.Mutex
				value := float32(0.1)
				return bind.MakeSetterFloat64(bind.New(&mu, &value))
			},
		},
	}
	// Number and Range embed the same InputFloat, so both must reconcile.
	widgets := []struct {
		kind string
		make func(bind.Setter[float64]) jaws.UI
	}{
		{"number", func(s bind.Setter[float64]) jaws.UI { return NewNumber(s) }},
		{"range", func(s bind.Setter[float64]) jaws.UI { return NewRange(s) }},
	}
	for _, w := range widgets {
		for _, tt := range tests {
			t.Run(w.kind+"/"+tt.name, func(t *testing.T) {
				jw, err := jaws.New()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(jw.Close)
				go jw.Serve()

				tr := jawstest.NewTestRequest(jw, nil)
				defer func() {
					tr.Close()
					<-tr.DoneCh
				}()
				<-tr.ReadyCh

				widget := w.make(tt.makeSetter())
				elem := tr.NewElement(widget)
				var buf strings.Builder
				if err = elem.JawsRender(&buf, []any{`step="any"`}); err != nil {
					t.Fatal(err)
				}
				if err = jaws.CallEventHandlers(elem.UI(), elem, what.Input, tt.input); err != nil {
					t.Fatalf("input %q: %v", tt.input, err)
				}

				select {
				case msg := <-tr.OutCh:
					if msg.What != what.Value || msg.Jid != elem.Jid() || msg.Data != tt.want {
						t.Fatalf("update = {%v %v %q}, want {%v %v %q}", msg.What, msg.Jid, msg.Data, what.Value, elem.Jid(), tt.want)
					}
				case <-time.After(time.Second):
					t.Fatal("no corrective value update received")
				}

				if err = jaws.CallEventHandlers(elem.UI(), elem, what.Input, tt.want); err != nil {
					t.Fatalf("unchanged input %q: %v", tt.want, err)
				}
				select {
				case msg := <-tr.OutCh:
					t.Fatalf("unchanged input produced update {%v %v %q}", msg.What, msg.Jid, msg.Data)
				case <-time.After(300 * time.Millisecond):
				}
			})
		}
	}
}

// TestInputFloat_TerminatesOnNonFiniteInput verifies that NaN/Inf from the untrusted
// browser terminates the Request and never reaches the bound value. This covers both
// the literals strconv.ParseFloat accepts ("NaN"/"Inf") and an overflowing magnitude
// like "1e999", which parses to ±Inf with a range error. JawsInput reports the event
// handled (nil).
func TestInputFloat_TerminatesOnNonFiniteInput(t *testing.T) {
	for _, bad := range []string{"NaN", "Inf", "-Inf", "+Inf", "1e999", "-1e999"} {
		t.Run(bad, func(t *testing.T) {
			_, rq := newCoreRequest(t)
			sf := newTestSetter(7.5)
			number := NewNumber(sf)
			elem, _ := renderUI(t, rq, number)
			if err := number.JawsInput(elem, bad); err != nil {
				t.Fatalf("JawsInput(%q) err = %v, want nil", bad, err)
			}
			if cause := context.Cause(rq.Context()); !errors.Is(cause, jaws.ErrValueNotFinite) {
				t.Fatalf("JawsInput(%q): cause = %v, want wrapping ErrValueNotFinite", bad, cause)
			}
			if sf.Get() != 7.5 {
				t.Fatalf("JawsInput(%q) mutated bound value to %v", bad, sf.Get())
			}
		})
	}
}

// TestInputFloat_TerminatesOnNonFiniteRender verifies that rendering a widget whose
// bound float64 is non-finite terminates the Request instead of emitting an
// unparseable "NaN"/"+Inf" literal or a coerced empty control.
func TestInputFloat_TerminatesOnNonFiniteRender(t *testing.T) {
	widgets := []struct {
		htmlType string
		make     func(bind.Setter[float64]) jaws.UI
	}{
		{"number", func(g bind.Setter[float64]) jaws.UI { return NewNumber(g) }},
		{"range", func(g bind.Setter[float64]) jaws.UI { return NewRange(g) }},
	}
	for _, w := range widgets {
		for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			_, rq := newCoreRequest(t)
			sf := newTestSetter(v)
			_, got := renderUI(t, rq, w.make(sf))
			if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
				t.Fatalf("%s rendered non-finite literal for %v: %s", w.htmlType, v, got)
			}
			if cause := context.Cause(rq.Context()); !errors.Is(cause, jaws.ErrValueNotFinite) {
				t.Fatalf("%s %v: cause = %v, want wrapping ErrValueNotFinite", w.htmlType, v, cause)
			}
		}
	}
}

func TestInputFloat_RegisterSendsInitialZeroValue(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	sf := newTestSetter(0.0)
	rw := RequestWriter{Request: tr.Request, Writer: tr.Recorder}
	id := rw.Register(NewNumber(sf))

	tr.InCh <- wire.WsMsg{} // wake the loop so the queued initial update can flush
	select {
	case <-time.After(300 * time.Millisecond):
		t.Fatal("no initial update received")
	case msg := <-tr.OutCh:
		if msg.What != what.Value || msg.Jid != id || msg.Data != "0" {
			t.Fatalf("initial update = {%v %v %q}, want {%v %v %q}", msg.What, msg.Jid, msg.Data, what.Value, id, "0")
		}
	}
}

// TestInputFloat_TerminatesOnNonFiniteUpdate verifies that a server-bound value going
// non-finite terminates the Request on the next update rather than being coerced or
// re-emitted.
func TestInputFloat_TerminatesOnNonFiniteUpdate(t *testing.T) {
	_, rq := newCoreRequest(t)
	sf := newTestSetter(1.5)
	number := NewNumber(sf)
	elem, _ := renderUI(t, rq, number)

	sf.Set(math.NaN())
	number.JawsUpdate(elem)
	if cause := context.Cause(rq.Context()); !errors.Is(cause, jaws.ErrValueNotFinite) {
		t.Fatalf("cause = %v, want wrapping ErrValueNotFinite", cause)
	}
}

func TestInputDateWidget(t *testing.T) {
	_, rq := newCoreRequest(t)
	d0, _ := time.Parse(assets.ISO8601, "2020-01-02")
	sd := newTestSetter(d0)

	date := NewDate(sd)
	elem, got := renderUI(t, rq, date, "dateattr")
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="date" value="2020-01-02" dateattr>$`, got)

	if err := date.JawsInput(elem, "2021-02-03"); err != nil {
		t.Fatal(err)
	}
	if sd.Get().Format(assets.ISO8601) != "2021-02-03" {
		t.Fatalf("unexpected date %v", sd.Get())
	}
	if err := date.JawsInput(elem, ""); err != nil {
		t.Fatal(err)
	}
	if !sd.Get().IsZero() {
		t.Fatalf("expected zero date for empty input, got %v", sd.Get())
	}
	if err := date.JawsInput(elem, "bad"); err == nil {
		t.Fatal("expected parse error")
	}
	d1, _ := time.Parse(assets.ISO8601, "2022-03-04")
	sd.Set(d1)
	date.JawsUpdate(elem)
}

// TestInputDate_BrowserEditNormalizesToMidnightUTC locks in the documented
// date-only behavior (issue #124): the control renders/reads a calendar date, so
// a browser edit resolves through time.Parse to midnight UTC and drops the bound
// value's time-of-day and location. Re-selecting the same calendar date from a
// non-UTC value still rewrites the bound value, because time.Time inequality
// includes the location.
func TestInputDate_BrowserEditNormalizesToMidnightUTC(t *testing.T) {
	_, rq := newCoreRequest(t)
	loc := time.FixedZone("CEST", 2*3600)
	sd := newTestSetter(time.Date(2026, 7, 18, 15, 30, 0, 0, loc))

	date := NewDate(sd)
	elem, _ := renderUI(t, rq, date, "dateattr")

	if err := date.JawsInput(elem, "2026-07-18"); err != nil {
		t.Fatal(err)
	}
	got := sd.Get()
	if want := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("date edit did not normalize to midnight UTC: got %v", got)
	}

	// A no-op re-selection of the same calendar date from a non-UTC value still
	// mutates the bound value to midnight UTC.
	sd.Set(time.Date(2026, 7, 18, 0, 0, 0, 0, loc))
	if err := date.JawsInput(elem, "2026-07-18"); err != nil {
		t.Fatal(err)
	}
	if sd.Get().Location() != time.UTC {
		t.Fatal("re-selecting the same date did not rewrite the bound value to UTC")
	}
}

// TestInputDate_NoSpuriousUpdateOnEqualDate guards the dedup fix: two time.Time
// values for the same calendar date but with different *Location pointers compare
// unequal under ==, yet render to the same ISO8601 string. JawsUpdate must dedup
// on the string, so the same-date update emits nothing and the next genuinely
// different date is the first thing the browser sees.
func TestInputDate_NoSpuriousUpdateOnEqualDate(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	go jw.Serve()
	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	// Same wall-clock date, different *Location pointer -> d0 != dSame under ==,
	// but both Format to "2020-01-02".
	d0 := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	dSame := time.Date(2020, 1, 2, 0, 0, 0, 0, time.FixedZone("UTC", 0))
	if d0 == dSame {
		t.Fatal("test precondition failed: values should be unequal under ==")
	}
	sd := newTestSetter(d0)

	date := NewDate(sd)
	elem := tr.NewElement(date)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	sd.Set(dSame)
	tr.BcastCh <- wire.Message{Dest: sd, What: what.Update} // must NOT emit
	sd.Set(time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC))
	tr.BcastCh <- wire.Message{Dest: sd, What: what.Update} // must emit "2099-12-31"

	select {
	case <-t.Context().Done():
		t.Fatal("no update received")
	case msg := <-tr.OutCh:
		if msg.What != what.Value || msg.Data != "2099-12-31" {
			t.Fatalf("first update = {%v %q}, want a single {Value \"2099-12-31\"} (a spurious same-date update leaked)", msg.What, msg.Data)
		}
	}
}

func TestInputMaybeDirtyErrValueUnchanged(t *testing.T) {
	_, rq := newCoreRequest(t)
	ss := newTestSetter("foo")
	text := NewText(ss)
	elem, _ := renderUI(t, rq, text)
	if err := text.JawsInput(elem, "foo"); err != nil {
		t.Fatalf("want nil got %v", err)
	}
}

// TestInputDirtyOnSetError asserts the revert-to-truth side effect of applyDirty:
// when JawsSet rejects an input with a real error, the input is both reported as
// an error AND marked dirty, so the next update pushes the server's value back to
// correct the client's optimistic display.
func TestInputDirtyOnSetError(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	tr := jawstest.NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer tr.Close()
	<-tr.ReadyCh

	ss := newTestSetter("server")
	text := NewText(ss)
	elem := tr.NewElement(text)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("rejected")
	ss.SetErr(wantErr)
	if err := text.JawsInput(elem, "client-typed"); !errors.Is(err, wantErr) {
		t.Fatalf("JawsInput error = %v, want %v", err, wantErr)
	}

	// The dirty mark must drive a corrective update carrying the server's truth.
	select {
	case <-t.Context().Done():
		t.Fatal("no corrective update received; input error did not mark the element dirty")
	case msg := <-tr.OutCh:
		if msg.What != what.Value || msg.Data != "server" {
			t.Fatalf("update = {%v %q}, want {Value \"server\"}", msg.What, msg.Data)
		}
	}
}

func TestTextarea_RenderEscapesHTML(t *testing.T) {
	_, rq := newCoreRequest(t)
	ss := newTestSetter(`x</textarea><script>alert("x")</script>`)

	_, got := renderUI(t, rq, NewTextarea(ss))
	mustMatch(t, `^<textarea id="Jid\.[0-9]+">\nx&lt;/textarea&gt;&lt;script&gt;alert\(&#34;x&#34;\)&lt;/script&gt;</textarea>$`, got)
}

func TestTextarea_RenderPreservesLeadingNewline(t *testing.T) {
	_, rq := newCoreRequest(t)
	ss := newTestSetter("\nhello")

	_, got := renderUI(t, rq, NewTextarea(ss))
	if want := "<textarea id=\"Jid.1\">\n\nhello</textarea>"; got != want {
		t.Fatalf("rendered textarea = %q, want %q", got, want)
	}
}

func TestInputTextWidget_RenderEscapesValueAttr(t *testing.T) {
	_, rq := newCoreRequest(t)
	value := `"&<>'\` + "\n"
	ss := newTestSetter(value)

	_, got := renderUI(t, rq, NewText(ss))
	want := "value=\"&#34;&amp;&lt;&gt;&#39;\\\n\""
	if !strings.Contains(got, want) {
		t.Fatalf("rendered input missing escaped value attr %q in %q", want, got)
	}
	if strings.Contains(got, `\"`) || strings.Contains(got, `\n`) {
		t.Fatalf("rendered input used Go/JavaScript-style escapes: %q", got)
	}
}

func TestInputTextWidget_InitialHTMLAttrFromBinder(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu deadlock.Mutex
	val := "foo"
	b := bind.New(&mu, &val).InitialHTMLAttr(func(bind.Binder[string], *jaws.Element) (s template.HTMLAttr) {
		s = `data-binder="yes"`
		return
	})

	_, got := renderUI(t, rq, NewText(b))
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="text" value="foo" data-binder="yes">$`, got)
}
