package ui

import (
	"errors"
	"html/template"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/what"
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
	_, got = renderUI(t, rq, textarea)
	mustMatch(t, `^<textarea id="Jid\.[0-9]+">quux</textarea>$`, got)
	textarea.JawsUpdate(elem)
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

func TestInputMaybeDirtyErrValueUnchanged(t *testing.T) {
	_, rq := newCoreRequest(t)
	ss := newTestSetter("foo")
	text := NewText(ss)
	elem, _ := renderUI(t, rq, text)
	if err := text.JawsInput(elem, "foo"); err != nil {
		t.Fatalf("want nil got %v", err)
	}
}

func TestTextarea_RenderEscapesHTML(t *testing.T) {
	_, rq := newCoreRequest(t)
	ss := newTestSetter(`x</textarea><script>alert("x")</script>`)

	_, got := renderUI(t, rq, NewTextarea(ss))
	mustMatch(t, `^<textarea id="Jid\.[0-9]+">x&lt;/textarea&gt;&lt;script&gt;alert\(&#34;x&#34;\)&lt;/script&gt;</textarea>$`, got)
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
