package ui

import (
	"errors"
	"html/template"
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

// TestInputDate_YearRangeRoundTrip locks in the documented year range (issue
// #204): the fixed-width "2006-01-02" layout round-trips only four-digit years.
// Years 1 and 9999 render as four digits and parse back through JawsInput, while
// year 10000 still renders (Format widens the year field to five digits) but a
// browser edit of that value fails to parse and leaves the bound value unchanged.
func TestInputDate_YearRangeRoundTrip(t *testing.T) {
	_, rq := newCoreRequest(t)
	sd := newTestSetter(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))

	// Rendering emits the calendar date verbatim, five-plus digits included.
	for _, iso := range []string{"0001-01-01", "9999-01-01", "10000-01-01"} {
		d, err := time.Parse(assets.ISO8601, iso)
		if err != nil {
			// time.Parse rejects five-digit years, so build year 10000 directly.
			d = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		sd.Set(d)
		_, got := renderUI(t, rq, NewDate(sd), "dateattr")
		mustMatch(t, `^<input id="Jid\.[0-9]+" type="date" value="`+iso+`" dateattr>$`, got)
	}

	// Four-digit years round-trip: JawsInput accepts them and updates the bind.
	sd.Set(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	date := NewDate(sd)
	elem, _ := renderUI(t, rq, date, "dateattr")
	for _, iso := range []string{"0001-01-01", "9999-01-02"} {
		if err := date.JawsInput(elem, iso); err != nil {
			t.Fatalf("year %q did not round-trip: %v", iso, err)
		}
		if got := sd.Get().Format(assets.ISO8601); got != iso {
			t.Fatalf("bound value = %q, want %q", got, iso)
		}
	}

	// A five-digit year fails to parse and leaves the last accepted value in place.
	before := sd.Get()
	if err := date.JawsInput(elem, "10000-01-01"); err == nil {
		t.Fatal("expected parse error for five-digit year")
	}
	if got := sd.Get(); !got.Equal(before) {
		t.Fatalf("bound value changed on rejected input: got %v, want %v", got, before)
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
