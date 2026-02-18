package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
)

func TestInputTextWidgets(t *testing.T) {
	_, rq := newRequest(t)
	ss := newTestSetter("foo")

	text := NewText(ss)
	elem, got := renderUI(t, rq, text)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="text" value="foo">$`, got)

	if err := text.JawsEvent(elem, what.Input, "bar"); err != nil {
		t.Fatal(err)
	}
	if ss.Get() != "bar" {
		t.Fatalf("want bar got %q", ss.Get())
	}
	if err := text.JawsEvent(elem, what.Click, "noop"); !errors.Is(err, core.ErrEventUnhandled) {
		t.Fatalf("want ErrEventUnhandled got %v", err)
	}
	ss.SetErr(errors.New("meh"))
	if err := text.JawsEvent(elem, what.Input, "omg"); err == nil || err.Error() != "meh" {
		t.Fatalf("want meh got %v", err)
	}
	ss.SetErr(nil)
	ss.Set("quux")
	text.JawsUpdate(elem)

	password := NewPassword(ss)
	_, got = renderUI(t, rq, password)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="password" value="quux">$`, got)

	textarea := NewTextarea(ss)
	_, got = renderUI(t, rq, textarea)
	mustMatch(t, `^<textarea id="Jid\.[0-9]+">quux</textarea>$`, got)
	textarea.JawsUpdate(elem)
}

func TestInputBoolWidgets(t *testing.T) {
	_, rq := newRequest(t)
	sb := newTestSetter(true)

	checkbox := NewCheckbox(sb)
	elem, got := renderUI(t, rq, checkbox)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="checkbox" checked>$`, got)
	if err := checkbox.JawsEvent(elem, what.Input, "false"); err != nil {
		t.Fatal(err)
	}
	if sb.Get() {
		t.Fatal("expected false")
	}
	if err := checkbox.JawsEvent(elem, what.Input, "bad"); err == nil {
		t.Fatal("expected parse error")
	}
	sb.Set(true)
	checkbox.JawsUpdate(elem)

	radio := NewRadio(sb)
	_, got = renderUI(t, rq, radio)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="radio" checked>$`, got)
}

func TestInputFloatWidgets(t *testing.T) {
	_, rq := newRequest(t)
	sf := newTestSetter(1.2)

	number := NewNumber(sf)
	elem, got := renderUI(t, rq, number)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="number" value="1.2">$`, got)
	if err := number.JawsEvent(elem, what.Input, "2.3"); err != nil {
		t.Fatal(err)
	}
	if sf.Get() != 2.3 {
		t.Fatalf("want 2.3 got %v", sf.Get())
	}
	if err := number.JawsEvent(elem, what.Input, "bad"); err == nil {
		t.Fatal("expected parse error")
	}
	sf.Set(3.4)
	number.JawsUpdate(elem)

	rng := NewRange(sf)
	_, got = renderUI(t, rq, rng)
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="range" value="3.4">$`, got)
}

func TestInputDateWidget(t *testing.T) {
	_, rq := newRequest(t)
	d0, _ := time.Parse(core.ISO8601, "2020-01-02")
	sd := newTestSetter(d0)

	date := NewDate(sd)
	elem, got := renderUI(t, rq, date, "dateattr")
	mustMatch(t, `^<input id="Jid\.[0-9]+" type="date" value="2020-01-02" dateattr>$`, got)

	if err := date.JawsEvent(elem, what.Input, "2021-02-03"); err != nil {
		t.Fatal(err)
	}
	if sd.Get().Format(core.ISO8601) != "2021-02-03" {
		t.Fatalf("unexpected date %v", sd.Get())
	}
	if err := date.JawsEvent(elem, what.Input, "bad"); err == nil {
		t.Fatal("expected parse error")
	}
	d1, _ := time.Parse(core.ISO8601, "2022-03-04")
	sd.Set(d1)
	date.JawsUpdate(elem)
}

func TestInputMaybeDirtyErrValueUnchanged(t *testing.T) {
	_, rq := newRequest(t)
	ss := newTestSetter("foo")
	text := NewText(ss)
	elem, _ := renderUI(t, rq, text)
	if err := text.JawsEvent(elem, what.Input, "foo"); err != nil {
		t.Fatalf("want nil got %v", err)
	}
}

func TestTextarea_RenderEscapesHTML(t *testing.T) {
	_, rq := newRequest(t)
	ss := newTestSetter(`x</textarea><script>alert("x")</script>`)

	_, got := renderUI(t, rq, NewTextarea(ss))
	mustMatch(t, `^<textarea id="Jid\.[0-9]+">x&lt;/textarea&gt;&lt;script&gt;alert\(&#34;x&#34;\)&lt;/script&gt;</textarea>$`, got)
}
