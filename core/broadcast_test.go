package core

import (
	"html/template"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

type testBroadcastTagGetter struct{}

func (testBroadcastTagGetter) JawsGetTag(*Request) any {
	return Tag("expanded")
}

func TestCoverage_GenerateHeadAndConvenienceBroadcasts(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	if err := jw.GenerateHeadHTML("%zz"); err == nil {
		t.Fatal("expected url parse error")
	}
	if err := jw.GenerateHeadHTML("/favicon.ico", "/app.js"); err != nil {
		t.Fatal(err)
	}

	jw.Reload()
	if msg := nextBroadcast(t, jw); msg.What != what.Reload {
		t.Fatalf("unexpected reload msg %#v", msg)
	}
	jw.Redirect("/next")
	if msg := nextBroadcast(t, jw); msg.What != what.Redirect || msg.Data != "/next" {
		t.Fatalf("unexpected redirect msg %#v", msg)
	}
	jw.Alert("info", "hello")
	if msg := nextBroadcast(t, jw); msg.What != what.Alert || msg.Data != "info\nhello" {
		t.Fatalf("unexpected alert msg %#v", msg)
	}

	jw.SetInner("t", template.HTML("<b>x</b>"))
	if msg := nextBroadcast(t, jw); msg.What != what.Inner || msg.Data != "<b>x</b>" {
		t.Fatalf("unexpected set inner msg %#v", msg)
	}
	jw.SetAttr("t", "k", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.SAttr || msg.Data != "k\nv" {
		t.Fatalf("unexpected set attr msg %#v", msg)
	}
	jw.RemoveAttr("t", "k")
	if msg := nextBroadcast(t, jw); msg.What != what.RAttr || msg.Data != "k" {
		t.Fatalf("unexpected remove attr msg %#v", msg)
	}
	jw.SetClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.SClass || msg.Data != "c" {
		t.Fatalf("unexpected set class msg %#v", msg)
	}
	jw.RemoveClass("t", "c")
	if msg := nextBroadcast(t, jw); msg.What != what.RClass || msg.Data != "c" {
		t.Fatalf("unexpected remove class msg %#v", msg)
	}
	jw.SetValue("t", "v")
	if msg := nextBroadcast(t, jw); msg.What != what.Value || msg.Data != "v" {
		t.Fatalf("unexpected set value msg %#v", msg)
	}
	jw.Insert("t", "0", "<i>a</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Insert || msg.Data != "0\n<i>a</i>" {
		t.Fatalf("unexpected insert msg %#v", msg)
	}
	jw.Replace("t", "0", "<i>b</i>")
	if msg := nextBroadcast(t, jw); msg.What != what.Replace || msg.Data != "0\n<i>b</i>" {
		t.Fatalf("unexpected replace msg %#v", msg)
	}
	jw.Delete("t")
	if msg := nextBroadcast(t, jw); msg.What != what.Delete {
		t.Fatalf("unexpected delete msg %#v", msg)
	}
	jw.Append("t", "<em>c</em>")
	if msg := nextBroadcast(t, jw); msg.What != what.Append || msg.Data != "<em>c</em>" {
		t.Fatalf("unexpected append msg %#v", msg)
	}
	jw.JsCall("t", "fn", `{"a":1}`)
	if msg := nextBroadcast(t, jw); msg.What != what.Call || msg.Data != `fn={"a":1}` {
		t.Fatalf("unexpected jscall msg %#v", msg)
	}
}

func TestBroadcast_ExpandsTagDestBeforeQueue(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tagger := testBroadcastTagGetter{}

	jw.Broadcast(Message{
		Dest: tagger,
		What: what.Inner,
		Data: "x",
	})
	msg := nextBroadcast(t, jw)
	if msg.What != what.Inner || msg.Data != "x" {
		t.Fatalf("unexpected msg %#v", msg)
	}
	if got, ok := msg.Dest.(Tag); !ok || got != Tag("expanded") {
		t.Fatalf("expected expanded Tag destination, got %T(%#v)", msg.Dest, msg.Dest)
	}

	jw.Broadcast(Message{
		Dest: []any{tagger, Tag("extra")},
		What: what.Value,
		Data: "v",
	})
	msg = nextBroadcast(t, jw)
	if msg.What != what.Value || msg.Data != "v" {
		t.Fatalf("unexpected msg %#v", msg)
	}
	dest, ok := msg.Dest.([]any)
	if !ok {
		t.Fatalf("expected []any destination, got %T(%#v)", msg.Dest, msg.Dest)
	}
	if len(dest) != 2 || dest[0] != Tag("expanded") || dest[1] != Tag("extra") {
		t.Fatalf("unexpected expanded destination %#v", dest)
	}

	jw.Broadcast(Message{
		Dest: "html-id",
		What: what.Delete,
	})
	msg = nextBroadcast(t, jw)
	if got, ok := msg.Dest.(string); !ok || got != "html-id" {
		t.Fatalf("expected raw html-id destination, got %T(%#v)", msg.Dest, msg.Dest)
	}
}

func TestBroadcast_PreservesMatchNoneDestination(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.Broadcast(Message{
		Dest: []any{},
		What: what.Update,
		Data: "x",
	})
	msg := nextBroadcast(t, jw)
	dest, ok := msg.Dest.([]any)
	if !ok {
		t.Fatalf("expected []any destination, got %T(%#v)", msg.Dest, msg.Dest)
	}
	if len(dest) != 0 {
		t.Fatalf("expected empty destination slice, got %#v", dest)
	}
}

func TestBroadcast_ReturnsWhenClosedAndQueueFull(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	jw.Broadcast(Message{What: what.Alert, Data: "info\nfirst"})
	jw.Close()

	done := make(chan struct{})
	go func() {
		jw.Broadcast(Message{What: what.Alert, Data: "info\nsecond"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("broadcast blocked after close")
	}

	msg := nextBroadcast(t, jw)
	if msg.Data != "info\nfirst" {
		t.Fatalf("unexpected queued message %#v", msg)
	}
	select {
	case extra := <-jw.bcastCh:
		t.Fatalf("unexpected extra message after close %#v", extra)
	default:
	}
}
